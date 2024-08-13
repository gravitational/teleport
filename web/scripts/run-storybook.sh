#!/bin/bash

if test -f web/certs/server.crt; then
  storybook dev -p 9002 -c web/.storybook --https --ssl-cert=web/certs/server.crt --ssl-key=web/certs/server.key "$@"
else
  echo \"Could not find SSL certificates. Please follow web/README.md to generate certificates.\" && false
fi
