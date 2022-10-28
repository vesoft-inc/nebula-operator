#!/usr/bin/env bash

set -uex

registry="${1:?registry}"
tag="${2:?tag}"
OS="${OS:-linux}"
ARCHES="${ARCHES:-amd64 arm64}"
IFS=" " read -r -a __arches__ <<<"$ARCHES"

images=()
for arch in "${__arches__[@]}"; do
  export GOARCH=${arch}
  make build
  image="${registry}:${tag}-${OS}-${arch}"
  docker build --no-cache --pull -t "${image}" --build-arg TARGETARCH="${GOARCH}" images/nebula-operator/
  images+=("${image}")
done

# images must be pushed to be referenced by docker manifest
for image in "${images[@]}"; do
    docker push "${image}"
done

export DOCKER_CLI_EXPERIMENTAL=enabled
docker manifest rm "${registry}:${tag}" || true
docker manifest create "${registry}:${tag}" "${images[@]}"
docker manifest push "${registry}:${tag}"
