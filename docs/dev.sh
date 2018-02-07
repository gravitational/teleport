#!/bin/bash

# IMPORTANT! To add a new version, say 8.1
#     * copy 2.3.yaml to 8.1.yaml
#     * edit 8.1.yaml
#     * edit theme/base.html and update docVersions variable

cd $(dirname $0)
./build.sh || exit $?

trap "exit" INT TERM ERR
trap "kill 0" EXIT

sass -C --sourcemap=none --watch theme/src/index.scss:theme/css/teleport-bundle.css &
mkdocs serve --livereload --config-file=latest.yaml --dev-addr=0.0.0.0:6600 &
wait



