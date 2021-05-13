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

VERSION_PKG=${VERSION_PKG:-github.com/vesoft-inc/nebula-operator/pkg/version}

function ldflag() {
  local key=${1}
  local val=${2}

  echo "-X '${VERSION_PKG}.${key}=${val}'"
}

function ldflags() {
  local gitCommit=$(git rev-parse "HEAD^{commit}" 2>/dev/null)
  local gitVersion=$(git describe --abbrev=16 --always --dirty=-dev)
  local gitTimestamp=$(git show -s --format=%ct HEAD^{commit})
  local buildTimestamp=$(date '+%s')

  local -a flags=($(ldflag "gitCommit" "${gitCommit}"))
  flags+=($(ldflag "gitVersion" "${gitVersion}"))
  flags+=($(ldflag "gitTimestamp" "${gitTimestamp}"))
  flags+=($(ldflag "buildTimestamp" "${buildTimestamp}"))

  echo "${flags[*]-}"
}

ldflags
