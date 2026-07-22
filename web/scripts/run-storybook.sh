#!/bin/bash

VITE_HTTPS_CERT="${VITE_HTTPS_CERT:-web/certs/server.crt}"
VITE_HTTPS_KEY="${VITE_HTTPS_KEY:-web/certs/server.key}"

if test -f $VITE_HTTPS_CERT; then
  # Generate the worker script for msw. This way we don't have to vendor in mockServiceWorker.js and
  # update it each time msw is updated.
  # See https://mswjs.io/docs/best-practices/managing-the-worker#committing-the-worker-script
  msw init web/.storybook/public --save

  storybook dev -p 9002 -c web/.storybook --https --ssl-cert=$VITE_HTTPS_CERT --ssl-key=$VITE_HTTPS_KEY "$@"
else
  echo \"Could not find SSL certificates. Please follow web/README.md to generate certificates.\" && false
fi
