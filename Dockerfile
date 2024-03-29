FROM alpine:3.18.2

ARG TARGETDIR
ARG USERNAME

RUN if [ "$USERNAME" = "ng-user" ]; then \
    addgroup -S -g 65532 ng-user && \
    adduser -S -D -H -u 65532 \
    -s /sbin/nologin -G ng-user -g ng-user ng-user; \
    fi

ADD bin/${TARGETDIR}/controller-manager /usr/local/bin/controller-manager
ADD bin/${TARGETDIR}/autoscaler /usr/local/bin/autoscaler
ADD bin/${TARGETDIR}/scheduler /usr/local/bin/scheduler

# [Optional] Set the default user. Omit if you want to keep the default as root.
USER $USERNAME
