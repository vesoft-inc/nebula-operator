package envfuncsext

import (
	"context"
	"k8s.io/klog/v2"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
)

func NebulaClusterReadyFuncForMetadResource(resource corev1.ResourceRequirements) NebulaClusterReadyFunc {
	return func(ctx context.Context, cfg *envconf.Config, nc *appsv1alpha1.NebulaCluster) (isReady bool, err error) {
		if !reflect.DeepEqual(nc.Spec.Metad.Resources, &resource) {
			klog.InfoS("Check Metad Resource but spec.metad.resources not expected",
				"requests", nc.Spec.Metad.Resources.Requests,
				"requestsExpected", resource.Requests,
				"limits", nc.Spec.Metad.Resources.Limits,
				"limitsExpected", resource.Limits,
			)
			return false, nil
		}

		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nc.MetadComponent().GetName(),
				Namespace: nc.Namespace,
			},
		}

		if err = cfg.Client().Resources().Get(ctx, sts.Name, sts.Namespace, sts); err != nil {
			klog.InfoS("Check Metad Resource but statefulset not found",
				"namespace", sts.Namespace,
				"name", sts.Name,
			)
			return false, nil
		}

		for _, c := range sts.Spec.Template.Spec.Containers {
			if c.Name != nc.MetadComponent().ComponentType().String() {
				continue
			}
			if reflect.DeepEqual(c.Resources, resource) {
				return true, nil
			} else {
				klog.InfoS("Check Metad Resource but metad sts's resource not expected",
					"requests", c.Resources.Requests,
					"requestsExpected", resource.Requests,
					"limits", c.Resources.Limits,
					"limitsExpected", resource.Limits,
				)
			}
		}

		return false, nil
	}
}

func NebulaClusterReadyFuncForStoragedResource(resource corev1.ResourceRequirements) NebulaClusterReadyFunc {
	return func(ctx context.Context, cfg *envconf.Config, nc *appsv1alpha1.NebulaCluster) (isReady bool, err error) {
		if !reflect.DeepEqual(nc.Spec.Storaged.Resources, &resource) {
			klog.InfoS("Check Storaged Resource but spec.storaged.resources not expected",
				"requests", nc.Spec.Storaged.Resources.Requests,
				"requestsExpected", resource.Requests,
				"limits", nc.Spec.Storaged.Resources.Limits,
				"limitsExpected", resource.Limits,
			)
			return false, nil
		}

		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nc.StoragedComponent().GetName(),
				Namespace: nc.Namespace,
			},
		}

		if err = cfg.Client().Resources().Get(ctx, sts.Name, sts.Namespace, sts); err != nil {
			klog.InfoS("Check Storaged Resource but statefulset not found",
				"namespace", sts.Namespace,
				"name", sts.Name,
			)
			return false, nil
		}

		for _, c := range sts.Spec.Template.Spec.Containers {
			if c.Name != nc.StoragedComponent().ComponentType().String() {
				continue
			}
			if reflect.DeepEqual(c.Resources, resource) {
				return true, nil
			} else {
				klog.InfoS("Check Storaged Resource but storaged sts's resource not expected",
					"requests", c.Resources.Requests,
					"requestsExpected", resource.Requests,
					"limits", c.Resources.Limits,
					"limitsExpected", resource.Limits,
				)
			}
		}

		return false, nil
	}
}

func NebulaClusterReadyFuncForGraphdResource(resource corev1.ResourceRequirements) NebulaClusterReadyFunc {
	return func(ctx context.Context, cfg *envconf.Config, nc *appsv1alpha1.NebulaCluster) (isReady bool, err error) {
		if !reflect.DeepEqual(nc.Spec.Graphd.Resources, &resource) {
			klog.InfoS("Check Graphd Resource but spec.graphd.resources not expected",
				"requests", nc.Spec.Graphd.Resources.Requests,
				"requestsExpected", resource.Requests,
				"limits", nc.Spec.Graphd.Resources.Limits,
				"limitsExpected", resource.Limits,
			)
			return false, nil
		}

		sts := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nc.GraphdComponent().GetName(),
				Namespace: nc.Namespace,
			},
		}

		if err = cfg.Client().Resources().Get(ctx, sts.Name, sts.Namespace, sts); err != nil {
			klog.InfoS("Check Graphd Resource but statefulset not found",
				"namespace", sts.Namespace,
				"name", sts.Name,
			)
			return false, nil
		}

		for _, c := range sts.Spec.Template.Spec.Containers {
			if c.Name != nc.GraphdComponent().ComponentType().String() {
				continue
			}
			if reflect.DeepEqual(c.Resources, resource) {
				return true, nil
			} else {
				klog.InfoS("Check Graphd Resource but graphd sts's resource not expected",
					"requests", c.Resources.Requests,
					"requestsExpected", resource.Requests,
					"limits", c.Resources.Limits,
					"limitsExpected", resource.Limits,
				)
			}
		}

		return false, nil
	}
}
