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

# You can see all supported e2e environment variables in "${ROOT}/tests/e2e/config/config.go"

export E2E_CLUSTER_KIND_CONFIG_PATH=${E2E_CLUSTER_KIND_CONFIG_PATH:-${ROOT}/hack/kind-config.yaml}

echo "e2e environment variables:"

env | grep '^E2E_' | sort

echo -e "\nstarting e2e tests\n"

go test -timeout 12h -v ./tests/e2e -args -test.timeout 12h -test.v -v 4 "${@}"
