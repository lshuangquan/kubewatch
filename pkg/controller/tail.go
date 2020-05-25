/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2020/05/21 10:06:14
 Desc     : tail 获取pod日志
*/

package controller

import (
	"bufio"
	"context"
	e "kubewatch/pkg/event"

	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
)

type Tail struct {
	Finished     bool
	Started      bool
	closed       chan struct{}
	logger       *logrus.Entry
	namespace    string
	pod          string
	container    string
	sinceSeconds int64
	tailLines    int64
	timestamps   bool
	queue        workqueue.RateLimitingInterface
}

// NewTail creates new Tail object
func NewTail(namespace, pod, container string, logger *logrus.Entry, queue workqueue.RateLimitingInterface,
	sinceSeconds int64, tailLines int64, timestamps bool) *Tail {
	return &Tail{
		Finished:     false,
		closed:       make(chan struct{}),
		logger:       logger,
		namespace:    namespace,
		pod:          pod,
		container:    container,
		sinceSeconds: sinceSeconds,
		tailLines:    tailLines,
		timestamps:   timestamps,
		queue:        queue,
	}
}

// Start starts Pod log streaming
func (t *Tail) Start(ctx context.Context, clientset kubernetes.Interface) {
	t.logger.Infof("Pod:%s Container:%s has been added", t.pod, t.container)
	go func() {
		rs, err := clientset.CoreV1().Pods(t.namespace).GetLogs(t.pod, &v1.PodLogOptions{
			Container:    t.container,
			Follow:       true,
			SinceSeconds: &t.sinceSeconds,
			Timestamps:   t.timestamps,
			TailLines:    &t.tailLines,
		}).Stream()
		if err != nil {
			t.logger.Println(err)
			return
		}
		defer rs.Close()

		go func() {
			<-t.closed
			rs.Close()
		}()

		sc := bufio.NewScanner(rs)
		t.Started = true
		for sc.Scan() {
			event := Event{}
			event.key = t.namespace + "/" + t.pod
			event.eventType = "create"
			event.resourceType = "log"
			event.namespace = t.namespace

			newEvent := e.Event{}
			newEvent.Namespace = t.namespace
			newEvent.Name = t.pod
			newEvent.Kind = "log"
			newEvent.Action = "created"
			newEvent.KubeEvent = sc.Text()

			event.event = newEvent
			t.queue.Add(event)
		}
	}()

	go func() {
		<-ctx.Done()
		close(t.closed)
	}()
}

// Finish finishes Pod log streaming with Pod completion
func (t *Tail) Finish() {
	t.logger.Infof("Pod:%s Container:%s has been finished", t.pod, t.container)
	t.Finished = true
}

// Delete finishes Pod log streaming with Pod deletion
func (t *Tail) Delete() {
	t.logger.Infof("Pod:%s Container:%s has been deleted", t.pod, t.container)
	close(t.closed)
}
