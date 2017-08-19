#!/bin/bash

# IMPORTANT! To add a new version, say 8.1
#     * copy 2.3.yaml to 8.1.yaml
#     * edit 8.1.yaml
#     * edit theme/base.html and update docVersions variable

cd $(dirname $0)
./build.sh || exit $?

mkdocs serve --livereload --config-file=latest.yaml --dev-addr=0.0.0.0:6600
