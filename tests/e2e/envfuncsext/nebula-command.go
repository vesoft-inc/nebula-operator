package envfuncsext

import (
	"context"
	"fmt"
	"time"

	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	nebulaclient "github.com/vesoft-inc/nebula-go/v3"
	appsv1alpha1 "github.com/vesoft-inc/nebula-operator/apis/apps/v1alpha1"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/e2eutils"
)

type (
	NebulaCommandOptions struct {
		ClusterName      string
		ClusterNamespace string
		Username         string
		Password         string
		Space            string
	}

	nebulaCommandCtxKey struct {
		clusterNamespace string
		clusterName      string
	}

	NebulaCommandCtxValue struct {
		Results nebulaclient.ResultSet
	}
)

func GetNebulaCommandCtxValue(clusterNamespace, clusterName string, ctx context.Context) *NebulaCommandCtxValue {
	v := ctx.Value(nebulaCommandCtxKey{
		clusterNamespace: clusterNamespace,
		clusterName:      clusterName,
	})
	data, _ := v.(*NebulaCommandCtxValue)
	return data
}

func RunNGCommand(cmdCtx NebulaCommandOptions, cmd string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		nc := &appsv1alpha1.NebulaCluster{}
		err := cfg.Client().Resources().Get(ctx, cmdCtx.ClusterName, cmdCtx.ClusterNamespace, nc)
		if err != nil {
			klog.ErrorS(err, "Get NebulaCluster failed", "namespace", cmdCtx.ClusterNamespace, "name", cmdCtx.ClusterName)
			return ctx, err
		}

		podName := nc.GraphdComponent().GetPodName(0)
		thriftPort := int(nc.GraphdComponent().GetPort(appsv1alpha1.GraphdPortNameThrift))
		localPorts, stopChan, err := e2eutils.PortForward(
			e2eutils.WithRestConfig(cfg.Client().RESTConfig()),
			e2eutils.WithPod(nc.GetNamespace(), podName),
			e2eutils.WithAddress("localhost"),
			e2eutils.WithPorts(thriftPort),
		)
		if err != nil {
			klog.ErrorS(err, "Unable to run command. Port forward failed.",
				"namespace", nc.GetNamespace(),
				"name", podName,
				"ports", []int{thriftPort},
				"command", cmd,
			)
			return ctx, err
		}
		defer close(stopChan)

		pool, err := nebulaclient.NewSslConnectionPool(
			[]nebulaclient.HostAddress{{Host: "localhost", Port: localPorts[0]}},
			nebulaclient.PoolConfig{
				MaxConnPoolSize: 10,
			},
			nil,
			nebulaclient.DefaultLogger{},
		)
		if err != nil {
			klog.ErrorS(err, "Unable to run command. Create graph connection pool failed",
				"namespace", nc.GetNamespace(),
				"name", podName,
				"ports", []int{thriftPort},
				"command", cmd,
			)
			return ctx, err
		}
		defer pool.Close()

		session, err := pool.GetSession("root", "nebula")
		if err != nil {
			klog.ErrorS(err, "Unable to run command. Create graph connection session failed",
				"namespace", nc.GetNamespace(),
				"name", podName,
				"ports", []int{thriftPort},
				"command", cmd,
			)
			return ctx, err
		}
		defer session.Release()

		result, err := session.Execute(cmd)
		if err != nil {
			klog.ErrorS(err, "Unable to run command. Graph exec failed",
				"namespace", nc.GetNamespace(),
				"name", podName,
				"ports", []int{thriftPort},
				"command", cmd,
			)
			return ctx, err
		}

		if result != nil {
			return context.WithValue(ctx, nebulaCommandCtxKey{
				clusterNamespace: cmdCtx.ClusterNamespace,
				clusterName:      cmdCtx.ClusterName,
			}, &NebulaCommandCtxValue{
				Results: *result,
			}), nil
		}

		return ctx, nil
	}
}

func RunGetStats(ctx context.Context, cfg *envconf.Config, space string, ngOptions NebulaCommandOptions) (*nebulaclient.ResultSet, error) {
	ctx, err := RunNGCommand(ngOptions, fmt.Sprintf("USE %v;submit job stats", space))(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to submit stats job: %v", err)
	}

	time.Sleep(2 * time.Second) //sleep 2 seconds to allow stats job to finish running

	ctx, err = RunNGCommand(ngOptions, fmt.Sprintf("USE %v;show stats", space))(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to show stats: %v", err)
	}

	commandContext := GetNebulaCommandCtxValue(ngOptions.ClusterNamespace, ngOptions.ClusterName, ctx)

	return &commandContext.Results, nil
}

func NGResultsEqual(resA, resB nebulaclient.ResultSet) bool {
	if resA.IsEmpty() || resB.IsEmpty() {
		return false
	}

	arrA := resA.AsStringTable()
	arrB := resB.AsStringTable()

	if len(arrA) != len(arrB) {
		return false
	}

	for i := 0; i < len(arrA); i++ {
		if len(arrA[i]) != len(arrB[i]) {
			return false
		}
		for j := 0; j < len(arrA[i]); j++ {
			if arrA[i][j] != arrB[i][j] {
				return false
			}
		}
	}

	return true
}

func PrintCommandResults(res nebulaclient.ResultSet) {
	arr := res.AsStringTable()

	for i := 0; i < len(arr); i++ {
		for j := 0; j < len(arr[i]); j++ {
			klog.V(4).Infof("%v \n", arr[i][j])
		}
		klog.V(4).Info()
	}
}
