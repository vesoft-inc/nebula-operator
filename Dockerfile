FROM --platform=$BUILDPLATFORM ubuntu:focal

ARG BUILDPLATFORM
ADD bin/${BUILDPLATFORM}/controller-manager /usr/local/bin/controller-manager
ADD bin/${BUILDPLATFORM}/scheduler /usr/local/bin/scheduler
USER 65532:65532
