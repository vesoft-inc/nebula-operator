# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:generateEmbeddedObjectMeta=false,maxDescLen=0"
# Set build symbols
LDFLAGS = $(if $(DEBUGGER),,-s -w) $(shell ./hack/version.sh)

DOCKER_REGISTRY ?= docker.io
DOCKER_REPO ?= ${DOCKER_REGISTRY}/vesoft
USERNAME ?= ng-user
IMAGE_TAG ?= v1.8.5
PROVISIONER_IMAGE_TAG ?= v0.1

CHARTS_VERSION ?= 1.8.5

export GO111MODULE := on
GOOS := $(if $(GOOS),$(GOOS),linux)
GOARCH := $(if $(GOARCH),$(GOARCH),amd64)
GOENV  := GO15VENDOREXPERIMENT="1" CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH)
GO     := $(GOENV) go
GO_BUILD := $(GO) build -trimpath
TARGETDIR := "$(GOOS)/$(GOARCH)"
# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: generate manifests build-operator

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(GOBIN)/controller-gen $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate: controller-gen defaulter-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(GOBIN)/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./apis/..."
	mkdir -p tmp.gen && $(GOBIN)/defaulter-gen --go-header-file "hack/boilerplate.go.txt" --input-dirs "./apis/autoscaling/v1alpha1" --output-base "./tmp.gen/"
	cp -f tmp.gen/apis/autoscaling/v1alpha1/zz_generated.defaults.go apis/autoscaling/v1alpha1/ && rm -rf ./tmp.gen

check: fmt vet lint ## Run check against code.

fmt:
	go fmt $(shell go list ./... | grep -v /vendor/)

vet:
	go vet $(shell go list ./... | grep -v /vendor/)

lint: golangci-lint
	$(GOBIN)/golangci-lint run

ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: manifests generate check ## Run unit-tests.
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.8.0/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test ./pkg/... -coverprofile cover.out

##@ e2e
e2e: kind ## Run e2e test.
	PATH="${GOBIN}:${PATH}" ./hack/e2e.sh $(E2EARGS)

##@ Build
build-operator: ## Build operator related binary.
	$(GO_BUILD) -ldflags '$(LDFLAGS)' -o bin/$(TARGETDIR)/controller-manager cmd/controller-manager/main.go
	$(GO_BUILD) -ldflags '$(LDFLAGS)' -o bin/$(TARGETDIR)/autoscaler cmd/autoscaler/main.go
	$(GO_BUILD) -ldflags '$(LDFLAGS)' -o bin/$(TARGETDIR)/scheduler cmd/scheduler/main.go

build-provisioner: ## Build provisioner binary.
	$(GO_BUILD) -ldflags '$(LDFLAGS)' -o bin/$(TARGETDIR)/local-pv-provisioner cmd/provisioner/main.go

helm-charts: ## Build helm charts.
	helm package charts/nebula-operator --version $(CHARTS_VERSION) --app-version $(CHARTS_VERSION)
	helm package charts/nebula-cluster --version $(CHARTS_VERSION) --app-version $(CHARTS_VERSION)
	mv nebula-operator-*.tgz nebula-cluster-*.tgz charts/
	helm repo index charts/ --url https://github.com/vesoft-inc/nebula-operator/releases/download/v$(CHARTS_VERSION)

docker-build: ## Build docker images.
	docker build --build-arg TARGETDIR=$(TARGETDIR) -t "${DOCKER_REPO}/nebula-operator:${IMAGE_TAG}" .

docker-push: ## Push docker images.
	docker push "${DOCKER_REPO}/nebula-operator:${IMAGE_TAG}"

ensure-buildx:
	chmod +x hack/init-buildx.sh && ./hack/init-buildx.sh

PLATFORMS = arm64 amd64
BUILDX_PLATFORMS = linux/arm64,linux/amd64

operator-multiarch: ensure-buildx ## Build and push the nebula-operator multiarchitecture docker images and manifest.
	$(foreach PLATFORM,$(PLATFORMS), echo -n "$(PLATFORM)..."; GOARCH=$(PLATFORM) make build-operator;)
	echo "Building and pushing nebula-operator image... $(BUILDX_PLATFORMS)"
	docker buildx build \
    		--no-cache \
    		--pull \
    		--push \
    		--progress plain \
    		--platform $(BUILDX_PLATFORMS) \
    		--file Dockerfile.multiarch \
    		--build-arg USERNAME=${USERNAME} \
    		-t "${DOCKER_REPO}/nebula-operator:${IMAGE_TAG}" .

alpine-multiarch: ## Build and push the alpine-tools docker images and manifest.
	echo "Building and pushing alpine-tools image... $(BUILDX_PLATFORMS)"
	docker buildx rm alpine-tools || true
	docker buildx create --driver-opt network=host --use --name=alpine-tools
	docker buildx build \
    		--no-cache \
    		--pull \
    		--push \
    		--progress plain \
    		--platform $(BUILDX_PLATFORMS) \
    		--file alpine.multiarch \
    		-t "${DOCKER_REPO}/nebula-alpine:latest" .

provisioner-multiarch: ## Build and push the local-pv-provisioner docker images and manifest.
	$(foreach PLATFORM,$(PLATFORMS), echo -n "$(PLATFORM)..."; GOARCH=$(PLATFORM) make build-provisioner;)
	echo "Building and pushing local-pv-provisioner image... $(BUILDX_PLATFORMS)"
	docker buildx rm provisioner || true
	docker buildx create --driver-opt network=host --use --name=provisioner
	docker buildx build \
    		--no-cache \
    		--pull \
    		--push \
    		--progress plain \
    		--platform $(BUILDX_PLATFORMS) \
    		--file provisioner.multiarch \
    		-t "${DOCKER_REPO}/local-pv-provisioner:${PROVISIONER_IMAGE_TAG}" .

##@ Deployment

install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(GOBIN)/kustomize build config/crd | kubectl apply -f -

uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(GOBIN)/kustomize build config/crd | kubectl delete -f -

deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(GOBIN)/kustomize edit set image controller=${DOCKER_REPO}/nebula-operator:${IMAGE_TAG}
	$(GOBIN)/kustomize build config/default | kubectl apply -f -

undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(GOBIN)/kustomize build config/default | kubectl delete -f -

##@ Tools

tools: golangci-lint controller-gen defaulter-gen kustomize ginkgo kind ## Download all go tools locally if necessary.

golangci-lint:
	@[ -f $(GOBIN)/golangci-lint ] || { \
	set -e ;\
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOBIN) v1.53.0 ;\
	}

controller-gen:
	$(call go-get-tool,$(GOBIN)/controller-gen,sigs.k8s.io/controller-tools/cmd/controller-gen@v0.15.0)

defaulter-gen:
	$(call go-get-tool,$(GOBIN)/default-gen,k8s.io/code-generator/cmd/defaulter-gen@v0.27.10)

kustomize:
	$(call go-get-tool,$(GOBIN)/kustomize,sigs.k8s.io/kustomize/kustomize/v4@v4.5.7)

ginkgo:
	$(call go-get-tool,$(GOBIN)/ginkgo,github.com/onsi/ginkgo/ginkgo@v1.16.5)

kind:
	$(call go-get-tool,$(GOBIN)/kind,sigs.k8s.io/kind@v0.20.0)

# go-get-tool will 'go get' any package $2 and install it to $1.
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
go get $(2) ;\
go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef
