/*
Copyright 2023 Vesoft Inc.

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

package envfuncsext

import (
	"bufio"
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strings"

	nebulago "github.com/vesoft-inc/nebula-go/v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/e2ematcher"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/e2eutils"
)

var defaultNebulaClusterReadyFuncs = []NebulaClusterReadyFunc{
	defaultNebulaClusterReadyFuncForStatus,
	defaultNebulaClusterReadyFuncForGraphd,
	defaultNebulaClusterReadyFuncForMetad,
	defaultNebulaClusterReadyFuncForStoraged,
	defaultNebulaClusterReadyFuncForPVC,
	defaultNebulaClusterReadyFuncForZone,
	defaultNebulaClusterReadyFuncForAgent,
	defaultNebulaClusterReadyFuncForExporter,
	defaultNebulaClusterReadyFuncForConsole,
}

func DefaultNebulaClusterReadyFunc(ctx context.Context, cfg *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
	for _, fn := range defaultNebulaClusterReadyFuncs {
		if isReady, err := fn(ctx, cfg, nc); !isReady || err != nil {
			return isReady, err
		}
	}
	return true, nil
}

func NebulaClusterReadyFuncForFields(ignoreValidationErrors bool, matchersMapping map[string]any) NebulaClusterReadyFunc {
	return func(_ context.Context, _ *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
		if err := e2ematcher.Struct(nc, matchersMapping); err != nil {
			if ignoreValidationErrors {
				klog.InfoS("Waiting for NebulaCluster to be ready but not expected", "err", err)
				return false, nil
			}

			klog.Error(err, "Waiting for NebulaCluster to be ready but not expected")
			return false, stderrors.New("check NebulaCluster")
		}
		return true, nil
	}
}

func defaultNebulaClusterReadyFuncForStatus(ctx context.Context, cfg *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
	isReady := nc.IsReady()
	if isReady {
		// TODO: Add more checks

		{ // Graphd status checks
			if !isComponentStatusExpected(
				&nc.Status.Graphd,
				&nc.Spec.Graphd.ComponentSpec,
				"namespace", nc.Namespace,
				"name", nc.Name,
				"component", appsv1alpha1.GraphdComponentType,
			) {
				isReady = false
			}
		}

		{ // Metad status checks
			if !isComponentStatusExpected(
				&nc.Status.Metad,
				&nc.Spec.Metad.ComponentSpec,
				"namespace", nc.Namespace,
				"name", nc.Name,
				"component", appsv1alpha1.MetadComponentType,
			) {
				isReady = false
			}
		}

		{ // Storaged status checks
			if !isComponentStatusExpected(
				&nc.Status.Storaged.ComponentStatus,
				&nc.Spec.Storaged.ComponentSpec,
				"namespace", nc.Namespace,
				"name", nc.Name,
				"component", appsv1alpha1.StoragedComponentType,
			) {
				isReady = false
			}

			if !nc.Status.Storaged.HostsAdded {
				klog.InfoS("Waiting for NebulaCluster to be ready but HostsAdded is false",
					"graphdReplicas", int(*nc.Spec.Graphd.Replicas),
					"namespace", nc.Namespace,
					"name", nc.Name,
					"component", appsv1alpha1.StoragedComponentType,
				)
			}
		}
	}

	return isReady, nil
}

func defaultNebulaClusterReadyFuncForGraphd(ctx context.Context, cfg *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
	isReady := true

	{
		if !isComponentStatefulSetExpected(ctx, cfg, nc.GraphdComponent()) {
			isReady = false
		}

		if !isComponentConfigMapExpected(ctx, cfg, nc.GraphdComponent()) {
			isReady = false
		}

		if !isComponentFlagsExpected(ctx, cfg, nc.GraphdComponent()) {
			isReady = false
		}
	}

	return isReady, nil
}

func defaultNebulaClusterReadyFuncForMetad(ctx context.Context, cfg *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
	isReady := true

	{
		if !isComponentStatefulSetExpected(ctx, cfg, nc.MetadComponent()) {
			isReady = false
		}

		if !isComponentConfigMapExpected(ctx, cfg, nc.MetadComponent()) {
			isReady = false
		}

		if !isComponentFlagsExpected(ctx, cfg, nc.MetadComponent()) {
			isReady = false
		}
	}

	return isReady, nil
}

func defaultNebulaClusterReadyFuncForStoraged(ctx context.Context, cfg *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
	isReady := true

	{
		if !isComponentStatefulSetExpected(ctx, cfg, nc.StoragedComponent()) {
			isReady = false
		}

		if !isComponentConfigMapExpected(ctx, cfg, nc.StoragedComponent()) {
			isReady = false
		}

		if !isComponentFlagsExpected(ctx, cfg, nc.StoragedComponent()) {
			isReady = false
		}
	}

	return isReady, nil
}

func defaultNebulaClusterReadyFuncForPVC(ctx context.Context, cfg *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
	pvcResourcesMappingExpected := make(map[string]string)
	for _, component := range []appsv1alpha1.NebulaClusterComponent{
		nc.GraphdComponent(),
		nc.MetadComponent(),
		nc.StoragedComponent(),
	} {
		volumeClaims, err := component.GenerateVolumeClaim()
		if err != nil {
			klog.ErrorS(err, "Waiting for NebulaCluster to be ready but pvc not expected(GenerateVolumeClaim failed)",
				"namespace", nc.GetNamespace(),
				"name", nc.GetName(),
				"component", component.ComponentType(),
			)
			return false, err
		}
		for i := range volumeClaims {
			volumeClaim := volumeClaims[i]
			for replicas := 0; replicas < int(component.ComponentSpec().Replicas()); replicas++ {
				pvcName := fmt.Sprintf("%s-%s-%d", volumeClaim.Name, component.GetName(), replicas)
				pvcResourcesMappingExpected[pvcName] = volumeClaim.Spec.Resources.Requests.Storage().String()
			}
		}
	}

	labelSelector := fmt.Sprintf("app.kubernetes.io/cluster=%s,app.kubernetes.io/component in (graphd, metad, storaged)", nc.Name)
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := cfg.Client().Resources().List(ctx, pvcs, resources.WithLabelSelector(labelSelector)); err != nil {
		klog.ErrorS(err, "Waiting for NebulaCluster to be ready but pvc not expected(list pvc failed)",
			"namespace", nc.GetNamespace(),
			"name", nc.GetName(),
		)
		return false, err
	}

	pvcResourcesMapping := make(map[string]string, len(pvcs.Items))
	for i := range pvcs.Items {
		pvc := pvcs.Items[i]
		pvcResourcesMapping[pvc.Name] = pvc.Spec.Resources.Requests.Storage().String()
	}

	if err := e2ematcher.DeepEqual(pvcResourcesMappingExpected).Match(pvcResourcesMapping); err != nil {
		klog.ErrorS(err, "Waiting for NebulaCluster to be ready but pvc not expected",
			"namespace", nc.GetNamespace(),
			"name", nc.GetName(),
		)
		return false, err
	}

	klog.V(5).InfoS("Waiting for NebulaCluster to be ready for pvc", "namespace", nc.GetNamespace(), "name", nc.GetName())

	return true, nil
}

func defaultNebulaClusterReadyFuncForZone(ctx context.Context, cfg *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
	metaConfig := nc.Spec.Metad.Config
	if metaConfig == nil || metaConfig["zone_list"] == "" {
		return true, nil
	}
	zones := strings.Split(metaConfig["zone_list"], ",")

	klog.V(5).InfoS("Waiting for NebulaCluster to be ready for zones", "namespace", nc.GetNamespace(), "name", nc.GetName(), "zones", zones)

	nodes := &corev1.NodeList{}
	labelSelector := fmt.Sprintf("topology.kubernetes.io/zone in (%s)", strings.Join(zones, ","))
	if err := cfg.Client().Resources().List(ctx, nodes, resources.WithLabelSelector(labelSelector)); err != nil {
		klog.ErrorS(err, "Waiting for NebulaCluster to be ready but zones not expected",
			"namespace", nc.GetNamespace(),
			"name", nc.GetName(),
			"zones", zones,
		)
		return false, err
	}
	nodeZoneMapping := make(map[string]string, len(nodes.Items))
	for i := range nodes.Items {
		node := nodes.Items[i]
		if lbs := node.GetLabels(); lbs != nil && lbs["topology.kubernetes.io/zone"] != "" {
			nodeZoneMapping[node.Name] = lbs["topology.kubernetes.io/zone"]
		}
	}

	labelSelector = fmt.Sprintf("app.kubernetes.io/cluster=%s,app.kubernetes.io/component in (graphd, storaged)", nc.Name)
	pods := &corev1.PodList{}
	if err := cfg.Client().Resources().List(ctx, pods, resources.WithLabelSelector(labelSelector)); err != nil {
		klog.ErrorS(err, "Waiting for NebulaCluster to be ready but zones not expected",
			"namespace", nc.GetNamespace(),
			"name", nc.GetName(),
			"zones", zones,
		)
		return false, err
	}

	podZoneMapping := make(map[string]string)
	componentZoneCountMapping := map[string]map[string]int{
		"graphd":   make(map[string]int, len(zones)),
		"storaged": make(map[string]int, len(zones)),
	}
	for i := range pods.Items {
		pod := pods.Items[i]
		componentName := pod.Labels["app.kubernetes.io/component"]
		zoneName, ok := nodeZoneMapping[pod.Spec.NodeName]
		if !ok {
			klog.ErrorS(nil, "Waiting for NebulaCluster to be ready but zones not expected(zone not found)",
				"namespace", nc.GetNamespace(),
				"name", nc.GetName(),
				"zones", zones,
				"podName", pod.Name,
				"nodeName", pod.Spec.NodeName,
			)
			return false, stderrors.New("zone not found")
		}
		podZoneMapping[pod.Name] = zoneName
		componentZoneCountMapping[componentName][zoneName] = componentZoneCountMapping[componentName][zoneName] + 1
	}

	for componentName, zoneCountMapping := range componentZoneCountMapping {
		minCount, maxCount := math.MaxInt, 0
		for _, count := range zoneCountMapping {
			if count < minCount {
				minCount = count
			}
			if count > maxCount {
				maxCount = count
			}
		}
		if minCount > maxCount || maxCount-minCount > 1 {
			klog.ErrorS(nil, "Waiting for NebulaCluster to be ready but zones not expected(zones not uneven)",
				"namespace", nc.GetNamespace(),
				"name", nc.GetName(),
				"zones", zones,
				"component", componentName,
				"zoneCountMapping", zoneCountMapping,
				"minCount", minCount,
				"maxCount", maxCount,
			)
			return false, stderrors.New("zones not uneven")
		}
	}

	// compare the zone in NebulaGraph
	podName := nc.GraphdComponent().GetPodName(0)
	thriftPort := int(nc.GraphdComponent().GetPort(appsv1alpha1.GraphdPortNameThrift))
	localPorts, stopChan, err := e2eutils.PortForward(
		e2eutils.WithRestConfig(cfg.Client().RESTConfig()),
		e2eutils.WithPod(nc.GetNamespace(), podName),
		e2eutils.WithAddress("localhost"),
		e2eutils.WithPorts(thriftPort),
	)
	if err != nil {
		klog.ErrorS(err, "Waiting for NebulaCluster to be ready but zones not expected(failed port forward)",
			"namespace", nc.GetNamespace(),
			"name", podName,
			"ports", []int{thriftPort},
		)
		return false, err
	}
	defer close(stopChan)

	pool, err := nebulago.NewSslConnectionPool(
		[]nebulago.HostAddress{{Host: "localhost", Port: localPorts[0]}},
		nebulago.PoolConfig{
			MaxConnPoolSize: 10,
		},
		nil,
		nebulago.DefaultLogger{},
	)
	if err != nil {
		klog.ErrorS(err, "Waiting for NebulaCluster to be ready but zones not expected(create graph connection pool failed)",
			"namespace", nc.GetNamespace(),
			"name", podName,
			"ports", []int{thriftPort},
		)
		return false, err
	}
	defer pool.Close()

	session, err := pool.GetSession("root", "nebula")
	if err != nil {
		klog.ErrorS(err, "Waiting for NebulaCluster to be ready but zones not expected(create graph connection session failed)",
			"namespace", nc.GetNamespace(),
			"name", podName,
			"ports", []int{thriftPort},
		)
		return false, err
	}
	defer session.Release()
	rs, err := session.Execute("SHOW ZONES")
	if err != nil {
		klog.ErrorS(err, "Waiting for NebulaCluster to be ready but zones not expected(graph exec failed)",
			"namespace", nc.GetNamespace(),
			"name", podName,
			"ports", []int{thriftPort},
			"statement", "SHOW ZONES",
		)
		return false, err
	}

	colNameMapping := make(map[string]int, rs.GetColSize())
	for i, colName := range rs.GetColNames() {
		colNameMapping[colName] = i
	}
	zoneIndex, okZoneIndex := colNameMapping["Name"]
	hostIndex, okHostIndex := colNameMapping["Host"]
	if !okZoneIndex || !okHostIndex {
		klog.ErrorS(err, "Waiting for NebulaCluster to be ready but zones not expected(graph ResultSet columns not found)",
			"namespace", nc.GetNamespace(),
			"name", podName,
			"ports", []int{thriftPort},
			"statement", "SHOW ZONES",
			"columns", []string{"Name", "Host"},
		)
		return false, err
	}

	podZoneMappingInNebulaCluster := make(map[string]string)
	for _, row := range rs.GetRows() {
		vals := row.GetValues()
		podZoneMappingInNebulaCluster[strings.Split(string(vals[hostIndex].GetSVal()), ".")[0]] = string(vals[zoneIndex].GetSVal())
	}

	if err = e2ematcher.Struct(podZoneMapping, e2ematcher.MapContainsMatchers(podZoneMappingInNebulaCluster)); err != nil {
		klog.ErrorS(err, "Waiting for NebulaCluster to be ready but component flags not expected(pod zone mismatch)",
			"namespace", nc.GetNamespace(),
			"name", podName,
			"ports", []int{thriftPort},
		)
		return false, err
	}

	klog.V(5).InfoS("Waiting for NebulaCluster to be ready for zones", "namespace", nc.GetNamespace(), "name", nc.GetName(), "zones mapping", componentZoneCountMapping)

	return true, nil
}

func defaultNebulaClusterReadyFuncForAgent(_ context.Context, _ *envconf.Config, _ *appsv1alpha1.NebulaCluster) (bool, error) {
	// TODO
	return true, nil
}

func defaultNebulaClusterReadyFuncForExporter(ctx context.Context, cfg *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
	if nc.Spec.Exporter == nil {
		return true, nil
	}
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-exporter", nc.GetName()),
			Namespace: nc.GetNamespace(),
		},
	}
	if err := cfg.Client().Resources().Get(ctx, deploy.Name, deploy.Namespace, deploy); err != nil {
		klog.InfoS("Check Exporter but Deployment not found",
			"namespace", deploy.Namespace,
			"name", deploy.Name,
		)
		return false, err
	}

	exporterContainerIdx := -1
	for i, c := range deploy.Spec.Template.Spec.Containers {
		if c.Name == "ng-exporter" {
			exporterContainerIdx = i
			break
		}
	}

	if exporterContainerIdx == -1 {
		klog.InfoS("Check Exporter but container not found",
			"namespace", deploy.Namespace,
			"name", deploy.Name,
			"container", "ng-exporter",
		)
		return false, stderrors.New("container not found")
	}

	if err := e2ematcher.Struct(
		deploy,
		map[string]any{
			"Spec": map[string]any{
				"Replicas": e2ematcher.ValidatorEq(*nc.Spec.Exporter.Replicas),
				"Template": map[string]any{
					"Spec": map[string]any{
						"Containers": map[string]any{
							fmt.Sprint(exporterContainerIdx): map[string]any{
								"Image":     e2ematcher.ValidatorEq(nc.ExporterComponent().ComponentSpec().PodImage()),
								"Resources": e2ematcher.DeepEqual(*nc.ExporterComponent().ComponentSpec().Resources()),
							},
						},
					},
				},
			},
			"Status": map[string]any{
				"ObservedGeneration": e2ematcher.ValidatorEq(deploy.Generation),
				"Replicas":           e2ematcher.ValidatorEq(*nc.Spec.Exporter.Replicas),
				"ReadyReplicas":      e2ematcher.ValidatorEq(*nc.Spec.Exporter.Replicas),
			},
		},
	); err != nil {
		klog.ErrorS(err, "Waiting for NebulaCluster to be ready but Exporter not expected",
			"namespace", deploy.Namespace,
			"name", deploy.Name,
		)
		return false, err
	}

	return true, nil
}

func defaultNebulaClusterReadyFuncForConsole(_ context.Context, _ *envconf.Config, nc *appsv1alpha1.NebulaCluster) (bool, error) {
	if nc.Spec.Console == nil {
		return true, nil
	}

	// TODO
	return true, nil
}

func isComponentStatusExpected(
	componentStatus *appsv1alpha1.ComponentStatus,
	componentSpec *appsv1alpha1.ComponentSpec,
	logKeysAndValues ...any,
) bool {
	if err := e2ematcher.Struct(
		componentStatus,
		map[string]any{
			"Version": e2ematcher.ValidatorEq(componentSpec.Version),
			"Phase":   e2ematcher.ValidatorEq(appsv1alpha1.RunningPhase),
			"Workload": map[string]any{
				"ReadyReplicas":     e2ematcher.ValidatorEq(*componentSpec.Replicas),
				"UpdatedReplicas":   e2ematcher.ValidatorEq(*componentSpec.Replicas),
				"CurrentReplicas":   e2ematcher.ValidatorEq(*componentSpec.Replicas),
				"AvailableReplicas": e2ematcher.ValidatorEq(*componentSpec.Replicas),
				"CurrentRevision":   e2ematcher.ValidatorEq(componentStatus.Workload.UpdateRevision),
			},
		},
	); err != nil {
		klog.ErrorS(err, "Waiting for NebulaCluster to be ready but componentStatus not expected", logKeysAndValues...)
		return false
	}

	return true
}

func isComponentStatefulSetExpected(ctx context.Context, cfg *envconf.Config, component appsv1alpha1.NebulaClusterComponent) bool {
	klog.V(5).InfoS("START isComponentStatefulSetExpected", "component", component.ComponentType())
	defer klog.V(5).InfoS("END   isComponentStatefulSetExpected", "component", component.ComponentType())
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      component.GetName(),
			Namespace: component.GetNamespace(),
		},
	}

	if err := cfg.Client().Resources().Get(ctx, sts.Name, sts.Namespace, sts); err != nil {
		klog.InfoS("Check Component Resource but StatefulSet not found",
			"namespace", sts.Namespace,
			"name", sts.Name,
		)
		return false
	}

	componentContainerIdx := -1
	for i, c := range sts.Spec.Template.Spec.Containers {
		if c.Name == component.ComponentType().String() {
			componentContainerIdx = i
			break
		}
	}

	if componentContainerIdx == -1 {
		klog.InfoS("Check Component Resource but container not found",
			"namespace", sts.Namespace,
			"name", sts.Name,
			"container", component.ComponentType().String(),
		)
		return false
	}

	if err := e2ematcher.Struct(
		sts,
		map[string]any{
			"ObjectMeta": map[string]any{
				"Name":      e2ematcher.ValidatorEq(component.GetName()),
				"Namespace": e2ematcher.ValidatorEq(component.GetNamespace()),
			},
			"Spec": map[string]any{
				"Replicas": e2ematcher.ValidatorEq(component.ComponentSpec().Replicas()),
				"Template": map[string]any{
					"Spec": map[string]any{
						"Containers": map[string]any{
							fmt.Sprint(componentContainerIdx): map[string]any{
								"Image":     e2ematcher.ValidatorEq(component.ComponentSpec().PodImage()),
								"Resources": e2ematcher.DeepEqual(*component.ComponentSpec().Resources()),
							},
						},
					},
				},
			},
			"Status": map[string]any{
				"ObservedGeneration": e2ematcher.ValidatorEq(sts.Generation),
				"Replicas":           e2ematcher.ValidatorEq(component.ComponentSpec().Replicas()),
				"ReadyReplicas":      e2ematcher.ValidatorEq(component.ComponentSpec().Replicas()),
				"CurrentReplicas":    e2ematcher.ValidatorEq(component.ComponentSpec().Replicas()),
				"UpdatedReplicas":    e2ematcher.ValidatorEq(component.ComponentSpec().Replicas()),
				"AvailableReplicas":  e2ematcher.ValidatorEq(component.ComponentSpec().Replicas()),
			},
		},
	); err != nil {
		klog.ErrorS(err, "Waiting for NebulaCluster to be ready but StatefulSet not expected",
			"namespace", sts.Namespace,
			"name", sts.Name,
		)
		return false
	}

	return true
}

func isComponentConfigMapExpected(ctx context.Context, cfg *envconf.Config, component appsv1alpha1.NebulaClusterComponent) bool {
	klog.V(5).InfoS("START isComponentConfigMapExpected", "component", component.ComponentType())
	defer klog.V(5).InfoS("END   isComponentConfigMapExpected", "component", component.ComponentType())

	componentConfigExpectedAll := component.GetConfig()

	// TODO: remove the filter
	// Filter out dynamic configuration
	componentConfigExpected := make(map[string]string, len(componentConfigExpectedAll))
	for k, v := range componentConfigExpectedAll {
		if _, ok := appsv1alpha1.DynamicFlags[k]; ok {
			continue
		}
		componentConfigExpected[k] = v
	}

	if len(componentConfigExpected) == 0 {
		return true
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      component.GetName(),
			Namespace: component.GetNamespace(),
		},
	}

	if err := cfg.Client().Resources().Get(ctx, cm.Name, cm.Namespace, cm); err != nil {
		klog.InfoS("Check Component Resource but ConfigMap not found",
			"namespace", cm.Namespace,
			"name", cm.Name,
		)
		return false
	}

	componentConfig, err := extractComponentConfig(
		strings.NewReader(cm.Data[component.GetConfigMapKey()]),
		regexp.MustCompile(`^--(\w+)=(.+)`),
	)
	if err != nil {
		klog.ErrorS(err, "failed to parse configmap configuration",
			"namespace", cm.Namespace,
			"name", cm.Name,
		)
		return false
	}

	if err := e2ematcher.Struct(componentConfig, e2ematcher.MapContainsMatchers(componentConfigExpected)); err != nil {
		klog.ErrorS(err, "Waiting for NebulaCluster to be ready but ConfigMap not expected",
			"namespace", cm.Namespace,
			"name", cm.Name,
		)
		return false
	}

	return true
}

func isComponentFlagsExpected(_ context.Context, cfg *envconf.Config, component appsv1alpha1.NebulaClusterComponent) bool {
	klog.V(5).InfoS("START isComponentFlagsExpected", "component", component.ComponentType())
	defer klog.V(5).InfoS("END   isComponentFlagsExpected", "component", component.ComponentType())

	componentConfigExpected := component.GetConfig()
	if len(componentConfigExpected) == 0 {
		return true
	}

	var httpPortName string
	switch component.ComponentType() {
	case appsv1alpha1.GraphdComponentType:
		httpPortName = appsv1alpha1.GraphdPortNameHTTP
	case appsv1alpha1.MetadComponentType:
		httpPortName = appsv1alpha1.GraphdPortNameHTTP
	case appsv1alpha1.StoragedComponentType:
		httpPortName = appsv1alpha1.StoragedPortNameHTTP
	default:
		klog.ErrorS(nil, "unknown component type", "component", component.ComponentType())
		return false
	}

	httpPort := int(component.GetPort(httpPortName))
	for ordinal := int32(0); ordinal < component.ComponentSpec().Replicas(); ordinal++ {
		if err := func() error {
			podName := component.GetPodName(ordinal)
			localPorts, stopChan, err := e2eutils.PortForward(
				e2eutils.WithRestConfig(cfg.Client().RESTConfig()),
				e2eutils.WithPod(component.GetNamespace(), podName),
				e2eutils.WithAddress("localhost"),
				e2eutils.WithPorts(httpPort),
			)
			if err != nil {
				klog.ErrorS(err, "failed port forward",
					"namespace", component.GetNamespace(),
					"name", podName,
					"ports", []int{httpPort},
				)
				return err
			}
			defer close(stopChan)

			resp, err := http.Get(fmt.Sprintf("http://localhost:%d/flags", localPorts[0]))
			if err != nil {
				klog.ErrorS(err, "failed to get flags",
					"namespace", component.GetNamespace(),
					"name", podName,
					"port", httpPort,
				)
				return err
			}
			defer func() {
				if err = resp.Body.Close(); err != nil {
					klog.ErrorS(err, "failed to close body",
						"namespace", component.GetNamespace(),
						"name", podName,
					)
				}
			}()

			componentConfig, err := extractComponentConfig(
				resp.Body,
				regexp.MustCompile(`^(\w+)=(.+)`),
			)
			if err != nil {
				klog.ErrorS(err, "failed to parse flags configuration ",
					"namespace", component.GetNamespace(),
					"name", podName,
				)
				return err
			}
			if err = e2ematcher.Struct(componentConfig, e2ematcher.MapContainsMatchers(componentConfigExpected)); err != nil {
				klog.ErrorS(err, "Waiting for NebulaCluster to be ready but component flags not expected",
					"namespace", component.GetNamespace(),
					"name", podName,
				)
				return err
			}
			return nil
		}(); err != nil {
			return false
		}
	}

	return true
}

func extractComponentConfig(r io.Reader, paramValuePattern *regexp.Regexp) (map[string]string, error) {
	componentConfig := make(map[string]string)
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		matches := paramValuePattern.FindStringSubmatch(scanner.Text())
		if len(matches) != 3 {
			continue
		}
		k, v := matches[1], matches[2]
		if l := len(v); l >= 2 && v[0] == '"' && v[l-1] == '"' {
			v = v[1 : l-1]
		}

		componentConfig[k] = v
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return componentConfig, nil
}
