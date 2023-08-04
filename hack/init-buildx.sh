#!/usr/bin/env bash

if [ -n "$DEBUG" ]; then
	set -x
fi

set -o errexit
set -o nounset
set -o pipefail

export DOCKER_CLI_EXPERIMENTAL=enabled

if ! docker buildx 2>&1 >/dev/null; then
  echo "buildx not available. Docker 19.03 or higher is required with experimental features enabled"
  exit 1
fi

if [ "$(uname)" == 'Linux' ]; then
  docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
fi

current_builder="$(docker buildx inspect)"
if ! grep -q "^Driver: docker$"  <<<"${current_builder}" && \
     grep -q "linux/amd64" <<<"${current_builder}" && \
     grep -q "linux/arm"   <<<"${current_builder}" && \
     grep -q "linux/arm64" <<<"${current_builder}" && \
     grep -q "linux/s390x" <<<"${current_builder}"; then
  exit 0
fi


docker buildx rm ng-operator || true
docker buildx create --driver-opt network=host --use --name=ng-operator