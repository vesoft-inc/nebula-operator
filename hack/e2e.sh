#!/usr/bin/env bash

# Copyright 2021 Vesoft Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

INSTALL_KUBERNETES=${INSTALL_KUBERNETES:-true}
UNINSTALL_KUBERNETES=${UNINSTALL_KUBERNETES:-true}
INSTALL_CERT_MANAGER=${INSTALL_CERT_MANAGER:-true}
INSTALL_CERT_MANAGER_VERSION=${INSTALL_CERT_MANAGER_VERSION:-v1.3.1}
INSTALL_KRUISE=${INSTALL_KRUISE:-true}
INSTALL_KRUISE_VERSION=${INSTALL_KRUISE_VERSION:-v0.8.1}
INSTALL_NEBULA_OPERATOR=${INSTALL_NEBULA_OPERATOR:-true}
KIND_NAME=${KIND_NAME:-e2e-test}
KIND_CONFIG=${KIND_CONFIG:-${ROOT}/hack/kind-config.yaml}
STORAGE_CLASS=${STORAGE_CLASS:-}
NEBULA_VERSION=${NEBULA_VERSION:-nightly}

if [[ "${INSTALL_KUBERNETES}" == "true" ]];then
  KUBECONFIG=~/.kube/${KIND_NAME}.kind.config
else
  KUBECONFIG=${KUBECONFIG:-~/.kube/config}
fi
DELETE_NAMESPACE=${DELETE_NAMESPACE:-true}
DELETE_NAMESPACE_ON_FAILURE=${DELETE_NAMESPACE_ON_FAILURE:-false}

echo "starting e2e tests"
echo "INSTALL_KUBERNETES: ${INSTALL_KUBERNETES}"
echo "UNINSTALL_KUBERNETES: ${UNINSTALL_KUBERNETES}"
echo "INSTALL_CERT_MANAGER: ${INSTALL_CERT_MANAGER}"
echo "INSTALL_CERT_MANAGER_VERSION: ${INSTALL_CERT_MANAGER_VERSION}"
echo "INSTALL_KRUISE: ${INSTALL_KRUISE}"
echo "INSTALL_KRUISE_VERSION: ${INSTALL_KRUISE_VERSION}"
echo "INSTALL_NEBULA_OPERATOR: ${INSTALL_NEBULA_OPERATOR}"
echo "KIND_NAME: ${KIND_NAME}"
echo "KIND_CONFIG: ${KIND_CONFIG}"
echo "STORAGE_CLASS: ${STORAGE_CLASS}"
echo "NEBULA_VERSION: ${NEBULA_VERSION}"

echo "KUBECONFIG: ${KUBECONFIG}"
echo "DELETE_NAMESPACE: ${DELETE_NAMESPACE}"
echo "DELETE_NAMESPACE_ON_FAILURE: ${DELETE_NAMESPACE_ON_FAILURE}"

ginkgo ./tests/e2e \
  -- \
  --install-kubernetes="${INSTALL_KUBERNETES}" \
  --uninstall-kubernetes="${UNINSTALL_KUBERNETES}" \
  --install-cert-manager="${INSTALL_CERT_MANAGER}" \
  --install-cert-manager-version="${INSTALL_CERT_MANAGER_VERSION}" \
  --install-kruise="${INSTALL_KRUISE}" \
  --install-kruise-version="${INSTALL_KRUISE_VERSION}" \
  --install-nebula-operator="${INSTALL_NEBULA_OPERATOR}" \
  --kind-name="${KIND_NAME}" \
  --kind-config="${KIND_CONFIG}" \
  --storage-class="${STORAGE_CLASS}" \
  --nebula-version="${NEBULA_VERSION}" \
  --kubeconfig="${KUBECONFIG}" \
  --delete-namespace="${DELETE_NAMESPACE}" \
  --delete-namespace-on-failure="${DELETE_NAMESPACE_ON_FAILURE}" \
  "${@}"
