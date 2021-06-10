/*
Copyright 2021 Vesoft Inc.

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

package extender

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	extender "k8s.io/kube-scheduler/extender/v1"

	"github.com/vesoft-inc/nebula-operator/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/scheduler/extender/predicates"
)

const (
	EventReason = "NebulaCluster Scheduling"

	NotBelongError = "the pod is not belong to nebula cluster"
)

type ScheduleExtender interface {
	Filter(*extender.ExtenderArgs) *extender.ExtenderFilterResult
}

type scheduleExtender struct {
	predicate predicates.Predicate
	recorder  record.EventRecorder
}

func NewScheduleExtender(kubeCli kubernetes.Interface, predicate predicates.Predicate) ScheduleExtender {
	eventBroadcaster := record.NewBroadcaster()
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "nebula-scheduler"})
	eventBroadcaster.StartLogging(func(format string, args ...interface{}) {
		getLog().Info(fmt.Sprintf(format, args...))
	})
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: typedcorev1.New(kubeCli.CoreV1().RESTClient()).Events(""),
		})
	return &scheduleExtender{predicate: predicate, recorder: recorder}
}

func (se *scheduleExtender) Filter(args *extender.ExtenderArgs) *extender.ExtenderFilterResult {
	log := getLog()
	log.Info("schedule pod", "namespace", args.Pod.Namespace, "name", args.Pod.Name)

	l := label.Label(args.Pod.Labels)

	if !l.IsManagedByNebulaOperator() && !l.IsNebulaComponent() {
		se.recorder.Event(args.Pod, corev1.EventTypeNormal, EventReason, NotBelongError)
		return &extender.ExtenderFilterResult{
			Nodes: args.Nodes,
		}
	}

	nodes, err := se.predicate.Filter(args.Pod, args.Nodes.Items)
	if err != nil {
		return &extender.ExtenderFilterResult{
			Error: err.Error(),
		}
	}

	return &extender.ExtenderFilterResult{
		Nodes: &corev1.NodeList{Items: nodes},
	}
}
