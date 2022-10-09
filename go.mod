module github.com/vesoft-inc/nebula-operator

go 1.17

require (
	github.com/facebook/fbthrift v0.31.1-0.20211129061412-801ed7f9f295
	github.com/gin-gonic/gin v1.6.3
	github.com/go-logr/logr v1.2.0
	github.com/google/go-cmp v0.5.5
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/openkruise/kruise-api v0.8.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.5.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/vesoft-inc/nebula-go/v3 v3.2.0
	go.uber.org/tools v0.0.0-20190618225709-2cfd321de3ee // indirect
	go.uber.org/zap v1.19.1
	k8s.io/api v0.23.5
	k8s.io/apimachinery v0.23.5
	k8s.io/apiserver v0.22.13
	k8s.io/cli-runtime v0.22.13
	k8s.io/client-go v0.23.5
	k8s.io/code-generator v0.22.13
	k8s.io/component-helpers v0.22.13 // indirect
	k8s.io/controller-manager v0.22.13 // indirect
	k8s.io/klog/v2 v2.30.0
	k8s.io/kube-scheduler v0.22.13
	k8s.io/kubectl v0.22.13
	k8s.io/kubernetes v1.22.13
	k8s.io/mount-utils v0.22.13 // indirect
	k8s.io/pod-security-admission v0.22.13 // indirect
	k8s.io/utils v0.0.0-20211116205334-6203023598ed
	sigs.k8s.io/controller-runtime v0.11.2
	sigs.k8s.io/yaml v1.3.0
)

replace (
	k8s.io/api => k8s.io/api v0.22.13
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.22.13
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.13
	k8s.io/apiserver => k8s.io/apiserver v0.22.13
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.22.13
	k8s.io/client-go => k8s.io/client-go v0.22.13
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.22.13
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.22.13
	k8s.io/code-generator => k8s.io/code-generator v0.22.13
	k8s.io/component-base => k8s.io/component-base v0.22.13
	k8s.io/component-helpers => k8s.io/component-helpers v0.22.13
	k8s.io/controller-manager => k8s.io/controller-manager v0.22.13
	k8s.io/cri-api => k8s.io/cri-api v0.22.13
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.22.13
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.22.13
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.22.13
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.22.13
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.22.13
	k8s.io/kubectl => k8s.io/kubectl v0.22.13
	k8s.io/kubelet => k8s.io/kubelet v0.22.13
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.22.13
	k8s.io/metrics => k8s.io/metrics v0.22.13
	k8s.io/mount-utils => k8s.io/mount-utils v0.22.13
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.22.13
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.22.13
)
