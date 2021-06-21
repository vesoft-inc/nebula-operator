# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true,preserveUnknownFields=false,maxDescLen=0"
# Set build symbols
LDFLAGS = $(if $(DEBUGGER),,-s -w) $(shell ./hack/version.sh)

DOCKER_REGISTRY ?= localhost:5000
DOCKER_REPO ?= ${DOCKER_REGISTRY}/vesoft
IMAGE_TAG ?= latest

export GO111MODULE := on
GOOS := $(if $(GOOS),$(GOOS),linux)
GOARCH := $(if $(GOARCH),$(GOARCH),amd64)
GOENV  := GO15VENDOREXPERIMENT="1" CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH)
GO     := $(GOENV) go
GO_BUILD := $(GO) build -trimpath
# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: build

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

manifests: $(GOBIN)/controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(GOBIN)/controller-gen $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate: $(GOBIN)/controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(GOBIN)/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."

check: tidy fmt vet imports lint

tidy:
	go mod tidy

fmt: $(GOBIN)/gofumpt
	# go fmt ./...
	$(GOBIN)/gofumpt -w -l ./

vet:
	go vet ./...

imports: $(GOBIN)/goimports $(GOBIN)/impi
	# $(GOBIN)/goimports -w -l -local github.com/vesoft-inc ./
	$(GOBIN)/impi --local github.com/vesoft-inc --scheme stdThirdPartyLocal -ignore-generated ./... ||( \
		echo "\nThere are some files need to reviser the imports." && \
		echo "1. Manual reviser." && \
		echo "2. Auto reviser all files with 'make imports-reviser', which will take a while." && \
		echo "3. Auto reviser the specified file via '$(GOBIN)/goimports-reviser -project-name github.com/vesoft-inc -rm-unused -file-path file-path'." && \
		exit 1 )

imports-reviser: $(GOBIN)/goimports-reviser
	find . -type f -name '*.go' -not -path "./vendor/*" -not -path "./hack/*" -exec \
		$(GOBIN)/goimports-reviser -project-name github.com/vesoft-inc -rm-unused -file-path {} \;

lint: $(GOBIN)/golangci-lint
	$(GOBIN)/golangci-lint run

ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: manifests generate check ## Run tests.
	mkdir -p ${ENVTEST_ASSETS_DIR}
	test -f ${ENVTEST_ASSETS_DIR}/setup-envtest.sh || curl -sSLo ${ENVTEST_ASSETS_DIR}/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.7.0/hack/setup-envtest.sh
	source ${ENVTEST_ASSETS_DIR}/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); go test ./pkg/... -coverprofile cover.out

##@ e2e
e2e: $(GOBIN)/ginkgo $(GOBIN)/kind helm
	PATH="${GOBIN}:${PATH}" ./hack/e2e.sh

##@ Build

build: generate check ## Build binary.
	$(GO_BUILD) -ldflags '$(LDFLAGS)' -o images/nebula-operator/bin/controller-manager cmd/controller-manager/main.go
	$(GO_BUILD) -ldflags '$(LDFLAGS)' -o images/nebula-operator/bin/scheduler cmd/scheduler/main.go

build-helm: helm
	helm repo index charts --url https://vesoft-inc.github.io/nebula-operator/charts
	helm package charts/nebula-operator
	helm package charts/nebula-cluster
	mv nebula-operator-*.tgz nebula-cluster-*.tgz charts/

run: run-controller-manager

run-controller-manager: manifests generate check
	go run -ldflags '$(LDFLAGS)' cmd/controller-manager/main.go

run-scheduler: manifests generate check
	go run -ldflags '$(LDFLAGS)' cmd/scheduler/main.go

docker-build: build test ## Build docker images.
	docker build -t "${DOCKER_REPO}/nebula-operator:${IMAGE_TAG}" images/nebula-operator/

docker-push: docker-build ## Push docker images.
	docker push "${DOCKER_REPO}/nebula-operator:${IMAGE_TAG}"

##@ Deployment

install: manifests $(GOBIN)/kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(GOBIN)/kustomize build config/crd | kubectl apply -f -

uninstall: manifests $(GOBIN)/kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(GOBIN)/kustomize build config/crd | kubectl delete -f -

deploy: manifests $(GOBIN)/kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(GOBIN)/kustomize edit set image controller=${DOCKER_REPO}/nebula-operator:${IMAGE_TAG}
	$(GOBIN)/kustomize build config/default | kubectl apply -f -

undeploy: $(GOBIN)/kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(GOBIN)/kustomize build config/default | kubectl delete -f -

##@ Tools
tools: $(GOBIN)/goimports \
	$(GOBIN)/impi \
	$(GOBIN)/goimports-reviser \
	$(GOBIN)/gofumpt \
	$(GOBIN)/golangci-lint \
	$(GOBIN)/controller-gen \
	$(GOBIN)/kustomize \
	$(GOBIN)/ginkgo \
	$(GOBIN)/kind \
	helm

$(GOBIN)/goimports:
	$(call go-get-tool,$(GOBIN)/goimports,golang.org/x/tools/cmd/goimports)

$(GOBIN)/impi:
	$(call go-get-tool,$(GOBIN)/impi,github.com/pavius/impi/cmd/impi)

$(GOBIN)/goimports-reviser:
	$(call go-get-tool,$(GOBIN)/goimports-reviser,github.com/incu6us/goimports-reviser/v2)

$(GOBIN)/gofumpt:
	$(call go-get-tool,$(GOBIN)/gofumpt,mvdan.cc/gofumpt)

$(GOBIN)/golangci-lint:
	@[ -f $(GOBIN)/golangci-lint ] || { \
	set -e ;\
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOBIN) v1.40.1 ;\
	}

$(GOBIN)/controller-gen:
	$(call go-get-tool,$(GOBIN)/controller-gen,sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1)

$(GOBIN)/kustomize:
	$(call go-get-tool,$(GOBIN)/kustomize,sigs.k8s.io/kustomize/kustomize/v3@v3.8.7)

$(GOBIN)/ginkgo:
	$(call go-get-tool,$(GOBIN)/kustomize,github.com/onsi/ginkgo/ginkgo@v1.16.2)

$(GOBIN)/kind:
	$(call go-get-tool,$(GOBIN)/kustomize,sigs.k8s.io/kind@v0.10.0)

helm:
	@[ -f /usr/local/bin/helm ] || { \
	set -e ;\
	curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash ;\
	}

# go-get-tool will 'go get' any package $2 and install it to $1.
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef
