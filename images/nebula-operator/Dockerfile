ARG TARGETARCH
# Declare stage using linux/amd64 base image
FROM --platform=linux/amd64 gcr.io/distroless/static:nonroot as stage-amd64
# Declare stage using linux/arm64 base image
FROM --platform=linux/arm64 gcr.io/distroless/static:nonroot as stage-arm64
FROM stage-${TARGETARCH} as final
ADD bin/controller-manager /usr/local/bin/controller-manager
ADD bin/scheduler /usr/local/bin/scheduler
USER 65532:65532
