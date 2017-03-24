#!/bin/bash
cd $(dirname $0)

# IMPORTANT! To add a new version, say 8.1
#     * copy 2.0.yaml to 8.1.yaml
#     * edit 8.1.yaml
#     * edit theme/base.html and update docVersions variable
mkdocs build --config-file 1.3.yaml
mkdocs build --config-file 2.0.yaml

# copy the index file which serves /docs requests and redirects
# visitors to the latest verion of QuickStart
cp index.html ../build/docs/index.html
