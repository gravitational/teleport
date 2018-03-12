#!/bin/bash

# IMPORTANT! To add a new version, say 8.1
#     * copy 2.3.yaml to 8.1.yaml
#     * edit 8.1.yaml
#     * edit theme/base.html and update docVersions variable

cd $(dirname $0)
rm -f latest.yaml

# build css files
sass -C --precision 9 --sourcemap=none theme/src/index.scss:theme/css/teleport-bundle.css

# will be set to the latest version after the loop below
doc_ver=""

for conf_file in $(ls *.yaml | sort); do
    doc_ver=${conf_file%.yaml}
    echo "Building docs version --> $doc_ver"
    mkdocs build --config-file $conf_file || exit $?
done

# drop the 'latest.yml' symlink to the latest version so `mkdocs serve` will
# automatically serve the latest
echo "Latest version --> $conf_file"
ln -fs $conf_file latest.yaml

# copy the index file which serves /docs requests and redirects
# visitors to the latest verion of QuickStart
cp index.html ../build/docs/index.html

# create a symlink to the latest 
cd ../build/docs
rm -f latest
ln -s $doc_ver latest
