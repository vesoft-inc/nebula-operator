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

package info

import (
	"bytes"
	"context"
	"fmt"
	"text/tabwriter"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/apis/pkg/label"
	"github.com/vesoft-inc/nebula-operator/pkg/ngctl/cmd/util/ignore"
)

type (
	NebulaClusterInfo struct {
		Name              string
		Namespace         string
		CreationTimestamp time.Time
		Components        []NebulaComponentInfo
		Service           NebulaServiceInfo
	}

	NebulaComponentInfo struct {
		Type          string
		Phase         string
		Replicas      int32
		ReadyReplicas int32
		CPU           resource.Quantity
		Memory        resource.Quantity
		Storage       resource.Quantity
		Image         string
	}

	NebulaServiceInfo struct {
		Type      corev1.ServiceType
		Port      int32
		Endpoints []*NebulaServiceEndpoints
	}

	NebulaServiceEndpoints struct {
		Desc      string
		Endpoints []string
	}
)

func NewNebulaClusterInfo(clusterName, namespace string, runtimeCli client.Client) (*NebulaClusterInfo, error) {
	var nc appsv1alpha1.NebulaCluster
	key := client.ObjectKey{Namespace: namespace, Name: clusterName}
	if err := runtimeCli.Get(context.TODO(), key, &nc); err != nil {
		return nil, err
	}

	var svc corev1.Service
	key = client.ObjectKey{Namespace: namespace, Name: nc.GraphdComponent().GetServiceName()}
	if err := runtimeCli.Get(context.TODO(), key, &svc); err != nil {
		return nil, err
	}

	selector, err := label.New().Cluster(clusterName).Graphd().Selector()
	if err != nil {
		return nil, err
	}
	var pods corev1.PodList
	listOptions := client.ListOptions{
		LabelSelector: selector,
		Namespace:     namespace,
	}
	if err := runtimeCli.List(context.TODO(), &pods, &listOptions); err != nil {
		return nil, err
	}

	graphd := NebulaComponentInfo{
		Phase:         string(nc.Status.Graphd.Phase),
		Replicas:      nc.Status.Graphd.Workload.Replicas,
		ReadyReplicas: nc.Status.Graphd.Workload.ReadyReplicas,
		Image:         nc.GraphdComponent().ComponentSpec().PodImage(),
	}
	if err := setComponentInfo(&graphd, nc.GraphdComponent()); err != nil {
		return nil, err
	}

	metad := NebulaComponentInfo{
		Phase:         string(nc.Status.Metad.Phase),
		Replicas:      nc.Status.Metad.Workload.Replicas,
		ReadyReplicas: nc.Status.Metad.Workload.ReadyReplicas,
	}
	if err := setComponentInfo(&metad, nc.MetadComponent()); err != nil {
		return nil, err
	}

	storage := NebulaComponentInfo{
		Phase:         string(nc.Status.Storaged.Phase),
		Replicas:      nc.Status.Storaged.Workload.Replicas,
		ReadyReplicas: nc.Status.Storaged.Workload.ReadyReplicas,
	}
	if err := setComponentInfo(&storage, nc.StoragedComponent()); err != nil {
		return nil, err
	}

	nci := &NebulaClusterInfo{
		Name:              clusterName,
		Namespace:         namespace,
		CreationTimestamp: nc.CreationTimestamp.Time,
		Components:        []NebulaComponentInfo{graphd, metad, storage},
		Service: NebulaServiceInfo{
			Type:      svc.Spec.Type,
			Port:      nc.GraphdComponent().GetPort(appsv1alpha1.GraphdPortNameThrift),
			Endpoints: nil,
		},
	}

	nci.appendEndpointsLoadBalancer(&svc)
	nci.appendEndpointsNodePort(&svc, &pods)
	nci.appendEndpointsService(&svc)
	nci.appendEndpointsClusterIP(&svc)

	return nci, nil
}

func (i *NebulaClusterInfo) Render() (string, error) {
	tw := new(tabwriter.Writer)
	buf := &bytes.Buffer{}
	tw.Init(buf, 0, 8, 2, ' ', 0)

	ignore.Fprintf(tw, "Name:\t%s\n", i.Name)
	ignore.Fprintf(tw, "Namespace:\t%s\n", i.Namespace)
	ignore.Fprintf(tw, "CreationTimestamp:\t%s\n", i.CreationTimestamp)
	ignore.Fprintf(tw, "Overview:\n")
	ignore.Fprintf(tw, "  \tPhase\tReady\tCPU\tMemory\tStorage\tVersion\n")
	ignore.Fprintf(tw, "  \t-----\t-----\t---\t------\t-------\t-------\n")

	for ix := range i.Components {
		c := &i.Components[ix]
		ignore.Fprintf(tw, "  %s:\t", c.Type)
		ignore.Fprintf(tw, "%s\t%d/%d\t%s\t%s\t%s\t%s\t\n",
			c.Phase, c.ReadyReplicas, c.Replicas, c.CPU.String(), c.Memory.String(), c.Storage.String(), c.Image)
	}

	ignore.Fprintf(tw, "Service(%s):\n", i.Service.Type)
	for _, sp := range i.Service.Endpoints {
		ignore.Fprintf(tw, "  %s\n", sp.Desc)
		for _, ep := range sp.Endpoints {
			ignore.Fprintf(tw, "    - %s\n", ep)
		}
	}

	_ = tw.Flush()
	return buf.String(), nil
}

func (i *NebulaClusterInfo) appendEndpointsLoadBalancer(svc *corev1.Service) {
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return
	}
	nse := NebulaServiceEndpoints{
		Desc: "Outer kubernetes: LoadBalance",
	}
	i.Service.Endpoints = append(i.Service.Endpoints, &nse)

	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		if ingress.Hostname != "" {
			nse.Endpoints = []string{fmt.Sprintf("%s:%d", ingress.Hostname, i.Service.Port)}
		}
		if ingress.IP != "" {
			nse.Endpoints = []string{fmt.Sprintf("%s:%d", ingress.IP, i.Service.Port)}
		}
	}

	if len(nse.Endpoints) == 0 {
		nse.Endpoints = []string{"Pending"}
	}
}

func (i *NebulaClusterInfo) appendEndpointsNodePort(svc *corev1.Service, pods *corev1.PodList) {
	if svc.Spec.Type != corev1.ServiceTypeNodePort {
		return
	}

	getNodePort := func() int32 {
		for _, p := range svc.Spec.Ports {
			if p.Name == appsv1alpha1.GraphdPortNameThrift {
				return p.NodePort
			}
		}
		return 0
	}
	nse := NebulaServiceEndpoints{
		Desc: "Outer kubernetes: NodePort",
	}
	i.Service.Endpoints = append(i.Service.Endpoints, &nse)
	nodePort := getNodePort()
	if nodePort == 0 {
		nse.Endpoints = []string{"No suitable port"}
		return
	}

	for ix := range pods.Items {
		pod := &pods.Items[ix]
		if pod.Status.Phase == corev1.PodRunning {
			nse.Endpoints = append(nse.Endpoints, fmt.Sprintf("%s:%d", pod.Status.HostIP, nodePort))
		}
	}
	if len(nse.Endpoints) == 0 {
		nse.Endpoints = []string{"No running pod"}
	}
}

func (i *NebulaClusterInfo) appendEndpointsService(svc *corev1.Service) {
	i.Service.Endpoints = append(i.Service.Endpoints, &NebulaServiceEndpoints{
		Desc:      "Inter kubernetes: Service",
		Endpoints: []string{fmt.Sprintf("%s.%s.svc:%d", svc.Name, svc.Namespace, i.Service.Port)},
	})
}

func (i *NebulaClusterInfo) appendEndpointsClusterIP(svc *corev1.Service) {
	if svc.Spec.ClusterIP == "" {
		return
	}
	i.Service.Endpoints = append(i.Service.Endpoints, &NebulaServiceEndpoints{
		Desc:      "Inter kubernetes: ClusterIP",
		Endpoints: []string{fmt.Sprintf("%s:%d", svc.Spec.ClusterIP, i.Service.Port)},
	})
}

func setComponentInfo(info *NebulaComponentInfo, c appsv1alpha1.NebulaClusterComponent) error {
	info.Type = string(c.ComponentType())
	if res := c.ComponentSpec().Resources(); res != nil {
		if cpu := res.Requests.Cpu(); cpu != nil {
			info.CPU = *cpu
		}
		if memory := res.Requests.Memory(); memory != nil {
			info.Memory = *memory
		}
	}
	if res := c.GetLogStorageResources(); res != nil {
		if storage := res.Requests.Storage(); storage != nil {
			info.Storage = *storage
		}
	}
	res, err := c.GetDataStorageResources()
	if err != nil {
		return err
	}
	if res != nil {
		if storage := res.Requests.Storage(); storage != nil {
			info.Storage.Add(*storage)
		}
	}
	info.Image = c.ComponentSpec().PodImage()

	return nil
}
