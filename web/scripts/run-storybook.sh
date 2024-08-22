#!/bin/bash

VITE_HTTPS_CERT="${VITE_HTTPS_CERT:-web/certs/server.crt}"
VITE_HTTPS_KEY="${VITE_HTTPS_KEY:-web/certs/server.key}"

if test -f $VITE_HTTPS_CERT; then
  storybook dev -p 9002 -c web/.storybook --https --ssl-cert=$VITE_HTTPS_CERT --ssl-key=$VITE_HTTPS_KEY "$@"
else
  echo \"Could not find SSL certificates. Please follow web/README.md to generate certificates.\" && false
fi
