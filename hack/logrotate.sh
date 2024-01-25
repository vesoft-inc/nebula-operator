#!/bin/env bash

ROTATE=5
SIZE=200M

if [ -n "${LOGROTATE_ROTATE}" ]; then
  ROTATE=${LOGROTATE_ROTATE}
fi

if [ -n "${LOGROTATE_SIZE}" ]; then
  SIZE=${LOGROTATE_SIZE}
fi

nebula="
/usr/local/nebula/logs/*.log
/usr/local/nebula/logs/*.impl
/usr/local/nebula/logs/*.INFO
/usr/local/nebula/logs/*.WARNING
/usr/local/nebula/logs/*.ERROR
{
        su root root
        daily
        rotate ${ROTATE}
        copytruncate
        nocompress
        missingok
        notifempty
        create 644 root root
        size ${SIZE}
}
"

echo "${nebula}" >/etc/logrotate.d/nebula
