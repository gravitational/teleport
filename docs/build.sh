#!/bin/bash

# IMPORTANT! To add a new version, say 8.1
#     * copy 2.3.yaml to 8.1.yaml
#     * edit 8.1.yaml
#     * edit theme/scripts.html and update docVersions variable

cd "$(dirname $0)" || exit
rm -f latest.yaml

# find all *.yaml files and convert them to array, pick the latest
cfiles=$(find . -maxdepth 1 -name '*.yaml' | sort)
cfiles_array=$(matfile -t array <<< "$cfiles")
latest_cfile=${cfiles_array[-1]} # becomes "3.1.yaml"
latest_ver=${latest_cfile%.yaml}         # becomes "3.1"

# build all documentation versions at the same time (4-8x speedup)
parallel --will-cite mkdocs build --config-file ::: $cfiles

# drop the 'latest.yml' symlink to the latest version so `mkdocs serve` will
# automatically serve the latest
echo "Latest version: $latest_ver"
ln -fs $latest_cfile latest.yaml

# copy the index file which serves /docs requests and redirects
# visitors to the latest verion of QuickStart
cp index.html ../build/docs/index.html

# create a symlink called 'latest' to the latest directory, like "3.1"
cd ../build/docs || exit
rm -f latest
ln -s $latest_ver latest

echo "The docs have been built and saved in 'build/docs'"
