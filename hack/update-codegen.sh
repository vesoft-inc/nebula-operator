#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

go mod vendor
retVal=$?
if [ $retVal -ne 0 ]; then
    exit $retVal
fi

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

echo "$(dirname "${BASH_SOURCE[0]}")/../../../.."

bash "${CODEGEN_PKG}/generate-groups.sh" all \
  github.com/vesoft-inc/nebula-operator/pkg/client \
  github.com/vesoft-inc/nebula-operator/apis \
  "apps:v1alpha1" \
  --output-base "$(dirname "${BASH_SOURCE[0]}")/../../../.." \
  --go-header-file "${SCRIPT_ROOT}/hack/boilerplate.go.txt"