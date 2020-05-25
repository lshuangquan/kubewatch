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

package event

import (
	"encoding/json"
	"kubewatch/pkg/utils"

	sparkoperator_v1beta2 "github.com/GoogleCloudPlatform/spark-on-k8s-operator/pkg/apis/sparkoperator.k8s.io/v1beta2"
	"github.com/sirupsen/logrus"
	apps_v1beta1 "k8s.io/api/apps/v1beta1"
	batch_v1 "k8s.io/api/batch/v1"
	api_v1 "k8s.io/api/core/v1"
	ext_v1beta1 "k8s.io/api/extensions/v1beta1"
)

// Event represent an event got from k8s api server
// Events from different endpoints need to be casted to KubewatchEvent
// before being able to be handled by handler
type Event struct {
	Namespace string
	Kind      string
	Host      string
	Name      string
	Action    string
	Status    string
	KubeEvent interface{}
}

var m = map[string]string{
	"created": "Normal",
	"deleted": "Danger",
	"updated": "Warning",
}

// New create new KubewatchEvent
func New(obj interface{}, action string) Event {
	var namespace, kind, host, name, status string
	objectMeta := utils.GetObjectMetaData(obj)
	namespace = objectMeta.Namespace
	name = objectMeta.Name
	status = m[action]
	kbEvent := Event{}
	switch object := obj.(type) {
	case *ext_v1beta1.DaemonSet:
		kind = "daemonset"
		name = object.Name
	case *apps_v1beta1.Deployment:
		kind = "deployment"
		name = object.Name
	case *batch_v1.Job:
		kind = "job"
	case *api_v1.Namespace:
		kind = "namespace"
	case *ext_v1beta1.Ingress:
		kind = "ingress"
		kbEvent.KubeEvent = object
	case *api_v1.PersistentVolume:
		kind = "persistentvolume"
	case *api_v1.Pod:
		kind = "pod"
		host = object.Spec.NodeName
	case *api_v1.ReplicationController:
		kind = "replicationcontroller"
	case *ext_v1beta1.ReplicaSet:
		kind = "replicaset"
		name = object.Name
	case *api_v1.Service:
		kind = "service"
		name = object.Name
		//component = string(object.Spec.Type)
	case *api_v1.Secret:
		kind = "secret"
	case *api_v1.ConfigMap:
		kind = "configmap"
	case *api_v1.Event:
		kind = "event"
	case *sparkoperator_v1beta2.SparkApplication:
		kind = "spark"
		name = object.Name
		namespace = object.Namespace
		// TODO: flink支持
	case Event:
		name = object.Name
		kind = object.Kind
		namespace = object.Namespace
	}
	kbEvent.KubeEvent = obj
	kbEvent.Status = status
	kbEvent.Namespace = namespace
	kbEvent.Kind = kind
	kbEvent.Host = host
	kbEvent.Name = name
	kbEvent.Action = action

	return kbEvent
}

// Message returns event message in standard format.
// included as a part of event packege to enhance code resuablity across handlers.
func (e *Event) Message() (msg string) {
	// using switch over if..else, since the format could vary based on the kind of the object in future.
	m, err := json.Marshal(e)
	if err != nil {
		logrus.Errorf("marshal event error: %v", err)
		return "nil"
	}
	return string(m)
	//switch e.Kind {
	//case "namespace":
	//	msg = fmt.Sprintf(
	//		"A namespace `%s` has been `%s`",
	//		e.Name,
	//		e.Action,
	//	)
	//default:
	//	msg = fmt.Sprintf(
	//		"A `%s-%s` in namespace `%s` has been `%s` with msg:`%s`",
	//		e.Kind,
	//		e.Name,
	//		e.Namespace,
	//		e.Action,
	//		e.KubeEvent,
	//	)
	//}
	//return msg
}
