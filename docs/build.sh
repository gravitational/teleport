#!/bin/bash

# IMPORTANT! To add a new version, say 8.1
#     * copy 2.3.yaml to 8.1.yaml
#     * edit 8.1.yaml
#     * edit theme/base.html and update docVersions variable

cd $(dirname $0)

for conf_file in $(ls *.yaml | sort); do
    echo "Building docs version --> $conf_file"
    mkdocs build --config-file $conf_file || exit $?
done

# drop the 'latest.yml' symlink to the latest version so `mkdocs serve` will
# automatically serve the latest
echo "Latest version --> $conf_file"
ln -fs $conf_file latest.yaml

# copy the index file which serves /docs requests and redirects
# visitors to the latest verion of QuickStart
cp index.html ../build/docs/index.html
