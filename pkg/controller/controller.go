/*
Copyright 2016 Skippbox, Ltd.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"kubewatch/config"
	"kubewatch/pkg/handlers"
	"kubewatch/pkg/utils"

	"github.com/sirupsen/logrus"

	sparkoperator_v1beta2 "github.com/GoogleCloudPlatform/spark-on-k8s-operator/pkg/apis/sparkoperator.k8s.io/v1beta2"
	sparkoperator "github.com/GoogleCloudPlatform/spark-on-k8s-operator/pkg/client/clientset/versioned"
	apps_v1 "k8s.io/api/apps/v1"
	batch_v1 "k8s.io/api/batch/v1"
	api_v1 "k8s.io/api/core/v1"
	ext_v1beta1 "k8s.io/api/extensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const maxRetries = 5
const logSecondsOffset = 100000000000000

var (
	serverStartTime time.Time
	sinceSeconds          = int64(math.Ceil(float64(logSecondsOffset) / float64(time.Second)))
	tailLines       int64 = 100
	tails                 = make(map[string]*Tail)
	added                 = make(chan *LogTarget, 100)
	finished              = make(chan *LogTarget, 100)
	deleted               = make(chan *LogTarget, 100)
)

// Event indicate the informerEvent
type Event struct {
	key          string
	eventType    string
	namespace    string
	resourceType string
	event        interface{}
	old          interface{}
}

type LogTarget struct {
	Namespace string
	Pod       string
	Container string
}

func (t *LogTarget) GetID() string {
	return t.Namespace + "_" + t.Pod + "_" + t.Container
}
func NewLogTarget(namespace, pod, container string) *LogTarget {
	return &LogTarget{
		Namespace: namespace,
		Pod:       pod,
		Container: container,
	}
}

// Controller object
type Controller struct {
	logger       *logrus.Entry
	clientset    kubernetes.Interface
	queue        workqueue.RateLimitingInterface
	informer     cache.SharedIndexInformer
	eventHandler handlers.Handler
}

// Start prepares watchers and run their controllers, then waits for process termination signals
func Start(conf *config.Config, eventHandler handlers.Handler) {
	var kubeClient kubernetes.Interface
	config, err := rest.InClusterConfig()
	if err != nil {
		config, kubeClient = utils.GetClientOutOfCluster()
	} else {
		kubeClient = utils.GetClient()
	}
	if conf.Resource.Pod {
		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.CoreV1().Pods(conf.Namespace).List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.CoreV1().Pods(conf.Namespace).Watch(options)
				},
			},
			&api_v1.Pod{},
			0, //Skip resync
			cache.Indexers{},
		)

		c := newResourceController(conf, kubeClient, eventHandler, informer, "pod")
		stopCh := make(chan struct{})
		defer close(stopCh)

		go c.Run(stopCh)
	}

	if conf.Resource.Event {
		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.CoreV1().Events(conf.Namespace).List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.CoreV1().Events(conf.Namespace).Watch(options)
				},
			},
			&api_v1.Event{},
			0, //Skip resync
			cache.Indexers{},
		)

		c := newResourceController(conf, kubeClient, eventHandler, informer, "event")
		stopCh := make(chan struct{})
		defer close(stopCh)

		go c.Run(stopCh)
	}

	if conf.Resource.DaemonSet {
		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.ExtensionsV1beta1().DaemonSets(conf.Namespace).List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.ExtensionsV1beta1().DaemonSets(conf.Namespace).Watch(options)
				},
			},
			&ext_v1beta1.DaemonSet{},
			0, //Skip resync
			cache.Indexers{},
		)

		c := newResourceController(conf, kubeClient, eventHandler, informer, "daemonset")
		stopCh := make(chan struct{})
		defer close(stopCh)

		go c.Run(stopCh)
	}

	if conf.Resource.ReplicaSet {
		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.ExtensionsV1beta1().ReplicaSets(conf.Namespace).List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.ExtensionsV1beta1().ReplicaSets(conf.Namespace).Watch(options)
				},
			},
			&ext_v1beta1.ReplicaSet{},
			0, //Skip resync
			cache.Indexers{},
		)

		c := newResourceController(conf, kubeClient, eventHandler, informer, "replicaset")
		stopCh := make(chan struct{})
		defer close(stopCh)

		go c.Run(stopCh)
	}

	if conf.Resource.Services {
		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.CoreV1().Services(conf.Namespace).List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.CoreV1().Services(conf.Namespace).Watch(options)
				},
			},
			&api_v1.Service{},
			0, //Skip resync
			cache.Indexers{},
		)

		c := newResourceController(conf, kubeClient, eventHandler, informer, "service")
		stopCh := make(chan struct{})
		defer close(stopCh)

		go c.Run(stopCh)
	}

	if conf.Resource.Deployment {
		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.AppsV1().Deployments(conf.Namespace).List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.AppsV1().Deployments(conf.Namespace).Watch(options)
				},
			},
			&apps_v1.Deployment{},
			0, //Skip resync
			cache.Indexers{},
		)

		c := newResourceController(conf, kubeClient, eventHandler, informer, "deployment")
		stopCh := make(chan struct{})
		defer close(stopCh)

		go c.Run(stopCh)
	}

	if conf.Resource.Namespace {
		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.CoreV1().Namespaces().List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.CoreV1().Namespaces().Watch(options)
				},
			},
			&api_v1.Namespace{},
			0, //Skip resync
			cache.Indexers{},
		)

		c := newResourceController(conf, kubeClient, eventHandler, informer, "namespace")
		stopCh := make(chan struct{})
		defer close(stopCh)

		go c.Run(stopCh)
	}

	if conf.Resource.ReplicationController {
		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.CoreV1().ReplicationControllers(conf.Namespace).List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.CoreV1().ReplicationControllers(conf.Namespace).Watch(options)
				},
			},
			&api_v1.ReplicationController{},
			0, //Skip resync
			cache.Indexers{},
		)

		c := newResourceController(conf, kubeClient, eventHandler, informer, "replication controller")
		stopCh := make(chan struct{})
		defer close(stopCh)

		go c.Run(stopCh)
	}

	if conf.Resource.Job {
		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.BatchV1().Jobs(conf.Namespace).List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.BatchV1().Jobs(conf.Namespace).Watch(options)
				},
			},
			&batch_v1.Job{},
			0, //Skip resync
			cache.Indexers{},
		)

		c := newResourceController(conf, kubeClient, eventHandler, informer, "job")
		stopCh := make(chan struct{})
		defer close(stopCh)

		go c.Run(stopCh)
	}

	if conf.Resource.PersistentVolume {
		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.CoreV1().PersistentVolumes().List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.CoreV1().PersistentVolumes().Watch(options)
				},
			},
			&api_v1.PersistentVolume{},
			0, //Skip resync
			cache.Indexers{},
		)

		c := newResourceController(conf, kubeClient, eventHandler, informer, "persistent volume")
		stopCh := make(chan struct{})
		defer close(stopCh)

		go c.Run(stopCh)
	}

	if conf.Resource.Secret {
		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.CoreV1().Secrets(conf.Namespace).List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.CoreV1().Secrets(conf.Namespace).Watch(options)
				},
			},
			&api_v1.Secret{},
			0, //Skip resync
			cache.Indexers{},
		)

		c := newResourceController(conf, kubeClient, eventHandler, informer, "secret")
		stopCh := make(chan struct{})
		defer close(stopCh)

		go c.Run(stopCh)
	}

	if conf.Resource.ConfigMap {
		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.CoreV1().ConfigMaps(conf.Namespace).List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.CoreV1().ConfigMaps(conf.Namespace).Watch(options)
				},
			},
			&api_v1.ConfigMap{},
			0, //Skip resync
			cache.Indexers{},
		)

		c := newResourceController(conf, kubeClient, eventHandler, informer, "configmap")
		stopCh := make(chan struct{})
		defer close(stopCh)

		go c.Run(stopCh)
	}

	if conf.Resource.Ingress {
		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.ExtensionsV1beta1().Ingresses(conf.Namespace).List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.ExtensionsV1beta1().Ingresses(conf.Namespace).Watch(options)
				},
			},
			&ext_v1beta1.Ingress{},
			0, //Skip resync
			cache.Indexers{},
		)

		c := newResourceController(conf, kubeClient, eventHandler, informer, "ingress")
		stopCh := make(chan struct{})
		defer close(stopCh)

		go c.Run(stopCh)
	}

	if conf.Resource.Spark {
		clientSet, err := sparkoperator.NewForConfig(config)
		if err != nil {
			panic(err)
		}
		informer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return clientSet.SparkoperatorV1beta2().SparkApplications(conf.Namespace).List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return clientSet.SparkoperatorV1beta2().SparkApplications(conf.Namespace).Watch(options)
				},
			},
			&sparkoperator_v1beta2.SparkApplication{},
			0, //Skip resync
			cache.Indexers{},
		)

		c := newResourceController(conf, kubeClient, eventHandler, informer, "spark")
		stopCh := make(chan struct{})
		defer close(stopCh)

		go c.Run(stopCh)
	}

	if conf.Resource.Log {
		c := newResourceController(conf, kubeClient, eventHandler, nil, "log")
		stopCh := make(chan struct{})
		go c.Run(stopCh)
	}

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	signal.Notify(sigterm, syscall.SIGINT)
	<-sigterm
}

func newResourceController(conf *config.Config, client kubernetes.Interface, eventHandler handlers.Handler, informer cache.SharedIndexInformer, resourceType string) *Controller {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	var newEvent Event
	var err error

	controller := &Controller{
		logger:       logrus.WithField("pkg", "kubewatch-"+resourceType),
		clientset:    client,
		queue:        queue,
		eventHandler: eventHandler,
	}
	if informer != nil {
		informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				newEvent.key, err = cache.MetaNamespaceKeyFunc(obj)
				newEvent.eventType = "create"
				newEvent.resourceType = resourceType
				newEvent.event = obj
				newEvent.namespace = utils.GetObjectMetaData(obj).Namespace
				logrus.WithField("pkg", "kubewatch-"+resourceType).Infof("Processing add to %v: %s", resourceType, newEvent.key)
				if err == nil {
					queue.Add(newEvent)
				}
				if resourceType == "pod" {
					for _, filter := range conf.Kube.PodLogFilters {
						reg := regexp.MustCompile(filter)
						if reg.FindString(newEvent.key) != "" {
							pod := obj.(*api_v1.Pod)
							logrus.Infof("pod.Status.Phase is %s", pod.Status.Phase)
							if pod.Status.Phase == api_v1.PodRunning {
								for _, container := range pod.Spec.Containers {
									added <- NewLogTarget(pod.Namespace, pod.Name, container.Name)
								}
							}
							break
						}
					}
				}
			},
			UpdateFunc: func(old, new interface{}) {
				newEvent.key, err = cache.MetaNamespaceKeyFunc(old)
				newEvent.eventType = "update"
				newEvent.resourceType = resourceType
				newEvent.event = new
				newEvent.old = old
				newEvent.namespace = utils.GetObjectMetaData(new).Namespace
				logrus.WithField("pkg", "kubewatch-"+resourceType).Infof("Processing update to %v: %s", resourceType, newEvent.key)
				if err == nil {
					queue.Add(newEvent)
				}
				if resourceType == "pod" {
					for _, filter := range conf.Kube.PodLogFilters {
						reg := regexp.MustCompile(filter)
						if reg.FindString(newEvent.key) != "" {
							pod := new.(*api_v1.Pod)
							logrus.Infof("pod.Status.Phase is %s", pod.Status.Phase)
							switch pod.Status.Phase {
							case api_v1.PodRunning:
								for _, container := range pod.Spec.Containers {
									added <- NewLogTarget(pod.Namespace, pod.Name, container.Name)
								}
							case api_v1.PodSucceeded, api_v1.PodFailed:
								for _, container := range pod.Spec.Containers {
									finished <- NewLogTarget(pod.Namespace, pod.Name, container.Name)
								}
							}
							break
						}
					}
				}
			},
			DeleteFunc: func(obj interface{}) {
				newEvent.key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
				newEvent.eventType = "delete"
				newEvent.resourceType = resourceType
				newEvent.namespace = utils.GetObjectMetaData(obj).Namespace
				newEvent.event = obj
				logrus.WithField("pkg", "kubewatch-"+resourceType).Infof("Processing delete to %v: %s", resourceType, newEvent.key)
				if err == nil {
					queue.Add(newEvent)
				}
				if resourceType == "pod" {
					for _, filter := range conf.Kube.PodLogFilters {
						reg := regexp.MustCompile(filter)
						if reg.FindString(newEvent.key) != "" {
							pod := obj.(*api_v1.Pod)
							logrus.Infof("pod.Status.Phase is %s", pod.Status.Phase)
							for _, container := range pod.Spec.Containers {
								deleted <- NewLogTarget(pod.Namespace, pod.Name, container.Name)
							}
							break
						}
					}
				}
			},
		})
		controller.informer = informer
	}
	return controller
}

// Run starts the kubewatch controller
func (c *Controller) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	c.logger.Info("Starting kubewatch controller")

	if c.informer != nil {
		go c.informer.Run(stopCh)

		if !cache.WaitForCacheSync(stopCh, c.HasSynced) {
			utilruntime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
			return
		}
	} else {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			for target := range added {
				id := target.GetID()

				if _, ok := tails[id]; ok {
					continue
				}
				tail := NewTail(target.Namespace, target.Pod, target.Container, c.logger, c.queue, sinceSeconds, tailLines, false)
				tail.Start(ctx, c.clientset)
				for {
					time.Sleep(1 * time.Microsecond)
					if tail.Started {
						tails[id] = tail
						break
					}
				}
			}
		}()

		go func() {
			for target := range finished {
				id := target.GetID()

				if tails[id] == nil {
					continue
				}

				if tails[id].Finished {
					continue
				}

				tails[id].Finish()
			}
		}()

		go func() {
			for target := range deleted {
				id := target.GetID()

				if tails[id] == nil {
					continue
				}

				tails[id].Delete()
				delete(tails, id)
			}
		}()
	}
	c.logger.Info("Kubewatch controller synced and ready")

	wait.Until(c.runWorker, time.Second, stopCh)
}

// HasSynced is required for the cache.Controller interface.
func (c *Controller) HasSynced() bool {
	return c.informer.HasSynced()
}

// LastSyncResourceVersion is required for the cache.Controller interface.
func (c *Controller) LastSyncResourceVersion() string {
	return c.informer.LastSyncResourceVersion()
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
		// continue looping
	}
}

func (c *Controller) processNextItem() bool {
	newEvent, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(newEvent)
	err := c.processItem(newEvent.(Event))
	if err == nil {
		// No error, reset the ratelimit counters
		c.queue.Forget(newEvent)
	} else if c.queue.NumRequeues(newEvent) < maxRetries {
		c.logger.Errorf("Error processing %s (will retry): %v", newEvent.(Event).key, err)
		c.queue.AddRateLimited(newEvent)
	} else {
		// err != nil and too many retries
		c.logger.Errorf("Error processing %s (giving up): %v", newEvent.(Event).key, err)
		c.queue.Forget(newEvent)
		utilruntime.HandleError(err)
	}

	return true
}

/*
   - Enhance event creation using client-side cacheing machanisms - pending
   - Enhance the processItem to classify events - done
   - Send alerts correspoding to events - done
*/
func (c *Controller) processItem(newEvent Event) error {
	if c.informer != nil {
		_, _, err := c.informer.GetIndexer().GetByKey(newEvent.key)
		if err != nil {
			return fmt.Errorf("Error fetching object with key %s from store: %v", newEvent.key, err)
		}
	}
	//get object's metedata
	objectMeta := utils.GetObjectMetaData(newEvent.event)

	// process events based on its type
	switch newEvent.eventType {
	case "create":
		// compare CreationTimestamp and serverStartTime and alert only on latest events
		// Could be Replaced by using Delta or DeltaFIFO
		if objectMeta.CreationTimestamp.Sub(serverStartTime).Seconds() > 0 {
			c.eventHandler.ObjectCreated(newEvent.event)
			return nil
		}
	case "update":
		/* TODOs
		   - enahace update event processing in such a way that, it send alerts about what got changed.
		*/
		//kbEvent := event.Event{
		//	Kind: newEvent.resourceType,
		//	Name: newEvent.key,
		//}
		c.eventHandler.ObjectUpdated(newEvent.old, newEvent.event)
		return nil
	case "delete":
		c.eventHandler.ObjectDeleted(newEvent.event)
		return nil
	}
	return nil
}
