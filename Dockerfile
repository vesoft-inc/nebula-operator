FROM alpine:3.18.2

ARG TARGETDIR

RUN addgroup -S -g 65532 ng-user && \
    adduser -S -D -H -u 65532 \
    -s /sbin/nologin -G ng-user -g ng-user ng-user

ADD bin/${TARGETDIR}/controller-manager /usr/local/bin/controller-manager
ADD bin/${TARGETDIR}/scheduler /usr/local/bin/scheduler
USER ng-user
