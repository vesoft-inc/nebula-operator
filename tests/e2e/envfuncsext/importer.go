package envfuncsext

import (
	"context"
	stderrors "errors"
	"fmt"

	"gopkg.in/yaml.v3"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"

	importerconfig "github.com/vesoft-inc/nebula-importer/v4/pkg/config"
	configv3 "github.com/vesoft-inc/nebula-importer/v4/pkg/config/v3"
	"github.com/vesoft-inc/nebula-operator/tests/e2e/e2eassets"
)

type (
	ImporterOption  func(*ImporterOptions)
	ImporterOptions struct {
		Name              string
		Namespace         string
		Image             string
		Config            importerconfig.Configurator
		ConfigUpdateFuncs []func(importerconfig.Configurator)
		DataDir           string
		InitContainers    []corev1.Container
		WaitOptions       []wait.Option
	}
)

func DefaultImporterOptions() *ImporterOptions {
	return &ImporterOptions{
		Image:   "vesoft/nebula-importer:v4",
		DataDir: "/data",
	}
}

func (o *ImporterOptions) WithOptions(opts ...ImporterOption) *ImporterOptions {
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func WithImporterName(name string) ImporterOption {
	return func(o *ImporterOptions) {
		o.Name = name
	}
}

func WithImporterNamespace(namespace string) ImporterOption {
	return func(o *ImporterOptions) {
		o.Namespace = namespace
	}
}

func WithImporterConfig(importerConfig importerconfig.Configurator) ImporterOption {
	return func(o *ImporterOptions) {
		o.Config = importerConfig
	}
}

func WithImporterConfigUpdate(fns ...func(importerconfig.Configurator)) ImporterOption {
	return func(o *ImporterOptions) {
		o.ConfigUpdateFuncs = append(o.ConfigUpdateFuncs, fns...)
	}
}

func WithImporterClientAddress(address string) ImporterOption {
	return WithImporterConfigUpdate(func(importerConfig importerconfig.Configurator) {
		cv3, ok := importerConfig.(*configv3.Config)
		if !ok {
			panic(fmt.Sprintf("unknown importer config type %T", importerConfig))
		}
		cv3.Client.Address = address
	})
}

func WithImporterClientUserPassword(user, password string) ImporterOption {
	return WithImporterConfigUpdate(func(importerConfig importerconfig.Configurator) {
		cv3, ok := importerConfig.(*configv3.Config)
		if !ok {
			panic(fmt.Sprintf("unknown importer config type %T", importerConfig))
		}
		cv3.Client.User = user
		cv3.Client.Password = password
	})
}

func WithImporterInitContainers(initContainers ...corev1.Container) ImporterOption {
	return func(o *ImporterOptions) {
		o.InitContainers = append(o.InitContainers, initContainers...)
	}
}

func WithImporterWaitOptions(opts ...wait.Option) ImporterOption {
	return func(o *ImporterOptions) {
		o.WaitOptions = append(o.WaitOptions, opts...)
	}
}

func ImportLDBC(opts ...ImporterOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		importerConfigBytes, err := e2eassets.AssetsFS.ReadFile("assets/importer-ldbc-snb.v3.yaml")
		if err != nil {
			return ctx, err
		}

		importerConfig, err := importerconfig.FromBytes(importerConfigBytes)
		if err != nil {
			return ctx, err
		}

		o := DefaultImporterOptions().WithOptions(opts...)

		return ImportData(append([]ImporterOption{
			WithImporterConfig(importerConfig),
			WithImporterInitContainers(
				corev1.Container{
					Name:  "download-data",
					Image: "busybox",
					Command: []string{
						"sh",
						"-c",
						fmt.Sprintf(`set -ex
wget -O /tmp/sf01.tar.gz https://oss-cdn.nebula-graph.com.cn/ldbc/sf01.tar.gz
tar -zxvf /tmp/sf01.tar.gz -C %s
`, o.DataDir),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "importer-data",
							MountPath: o.DataDir,
						},
					},
				},
			),
		}, opts...)...)(ctx, cfg)
	}
}

func ImportData(opts ...ImporterOption) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		o := DefaultImporterOptions().WithOptions(opts...)

		for _, fn := range o.ConfigUpdateFuncs {
			fn(o.Config)
		}

		cm, job, err := o.generateImporterJob()
		if err != nil {
			return ctx, err
		}

		if err = cfg.Client().Resources().Create(ctx, cm); err != nil {
			return ctx, err
		}

		if err = cfg.Client().Resources().Create(ctx, job); err != nil {
			return ctx, err
		}

		if err = wait.For(func(ctx context.Context) (done bool, err error) {
			if err = cfg.Client().Resources().Get(ctx, job.GetName(), job.GetNamespace(), job); err != nil {
				klog.ErrorS(err, "Get importer job failed", "namespace", job.Namespace, "name", job.Name)
				return false, err
			}
			klog.V(4).InfoS("Waiting for importer job complete", "namespace", job.Namespace, "name", job.Name)

			for _, condition := range job.Status.Conditions {
				if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
					return true, nil
				}

				if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
					return false, stderrors.New("importer job failed")
				}
			}

			return false, nil
		}, o.WaitOptions...); err != nil {
			klog.ErrorS(err, "Waiting for importer job complete failed", "namespace", job.Namespace, "name", job.Name)
			return ctx, err
		}

		klog.InfoS("Waiting for importer job complete successfully", "namespace", job.Namespace, "name", job.Name)
		return ctx, nil
	}
}

func (o *ImporterOptions) generateImporterJob() (*corev1.ConfigMap, *batchv1.Job, error) {
	importerConfigYAML, err := yaml.Marshal(o.Config)
	if err != nil {
		return nil, nil, err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.Name + "-cfg",
			Namespace: o.Namespace,
		},
		Data: map[string]string{
			"importer.yaml": string(importerConfigYAML),
		},
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.Name,
			Namespace: o.Namespace,
		},
		Spec: batchv1.JobSpec{
			Parallelism:  pointer.Int32(1),
			Completions:  pointer.Int32(1),
			BackoffLimit: pointer.Int32(3),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					InitContainers: o.InitContainers,
					Containers: []corev1.Container{
						{
							Name:            "importer",
							Image:           o.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"/usr/local/bin/nebula-importer",
								"-c",
								"/importer.yaml",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "importer-cfg",
									MountPath: "/importer.yaml",
									SubPath:   "importer.yaml",
									ReadOnly:  true,
								},
								{
									Name:      "importer-data",
									MountPath: o.DataDir,
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "importer-cfg",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: cm.Name,
									},
								},
							},
						},
						{
							Name: "importer-data",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	return cm, job, nil
}
