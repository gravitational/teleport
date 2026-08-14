#!/bin/sh

if [ -f /certs/rootCA.pem ]; then
  cp /certs/rootCA.pem /usr/local/share/ca-certificates/teleport-root-ca.crt
fi


if [ -f /certs/ca.crt ]; then
  cp /certs/ca.crt /usr/local/share/ca-certificates/postgres-root-ca.crt
fi

update-ca-certificates
exec /usr/bin/dumb-init -- "$@"
