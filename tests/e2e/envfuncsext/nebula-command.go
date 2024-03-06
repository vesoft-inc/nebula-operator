package envfuncsext

import (
	"context"
	"fmt"
	"time"

	nebulaclient "github.com/vesoft-inc/nebula-go/v3"
	appspkg "github.com/vesoft-inc/nebula-operator/pkg/kube"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
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
		client, err := appspkg.NewClientSet(cfg.Client().RESTConfig())
		if err != nil {
			return ctx, fmt.Errorf("error getting kube clientset: %v", err)
		}

		nodes, err := client.Node().ListAllNodes()
		if err != nil {
			return ctx, fmt.Errorf("error listing nodes in the cluster: %v", err)
		}
		if len(nodes) == 0 {
			return ctx, fmt.Errorf("error listing nodes in the cluster: node list is empty")
		}

		service, err := client.Service().GetService(cmdCtx.ClusterNamespace, fmt.Sprintf("%v-graphd-svc", cmdCtx.ClusterName))
		if err != nil {
			return ctx, fmt.Errorf("error getting graphd service: %v", err)
		}

		ports := service.Spec.Ports

		var portNum int
		for _, port := range ports {
			if port.Name == "thrift" {
				portNum = int(port.NodePort)
			}
		}

		if portNum <= 0 {
			return ctx, fmt.Errorf("error getting service port: %v is <= 0", portNum)
		}

		hostAddress := nebulaclient.HostAddress{Host: nodes[0].Status.Addresses[0].Address, Port: portNum}

		config, err := nebulaclient.NewSessionPoolConf(
			cmdCtx.Username,
			cmdCtx.Password,
			[]nebulaclient.HostAddress{hostAddress},
			cmdCtx.Space,
		)
		if err != nil {
			return ctx, fmt.Errorf("error creating new session pool config: %v", err)
		}

		sessionPool, err := nebulaclient.NewSessionPool(*config, nebulaclient.DefaultLogger{})
		if err != nil {
			return ctx, fmt.Errorf("error creating new session pool: %v", err)
		}

		result, err := sessionPool.Execute(cmd)
		if err != nil {
			return ctx, fmt.Errorf("error running command \"%v\": %v", cmd, err)
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
