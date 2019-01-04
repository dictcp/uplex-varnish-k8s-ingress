#!/bin/bash

set -e
set -u

exec /usr/sbin/varnishd -F -a :${HTTP_PORT},${PROTO} -a k8s=:${READY_PORT} \
     -S ${SECRET_PATH}/${SECRET_FILE} -T 0.0.0.0:${ADMIN_PORT}             \
     -p vcl_path=/etc/varnish -I /etc/varnish/start.cli	-f '' "$@"
