#!/bin/bash
#
# Example of how consul must be started in the full TLS mode, i.e.
#   - server cert is checked by clients
#   - client cert is checked by the server
#
# NOTE: this file is also used to run consul tests.
#
HERE=$(readlink -f $0)
cd $(dirname $HERE)

mkdir -p data
consul agent -dev \
  -bind=127.0.0.1 \
  -data-dir data/consul \
  -config-file=consul-config.json
