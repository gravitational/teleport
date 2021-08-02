#!/bin/bash
if [ -z "$1" ]
    then echo "Usage: $(basename $0) <version>" 1>&2; exit 1
fi

VERSION=$1
MAJ_VER=$(echo ${VERSION} | cut -d "." -f1)
MAJ_VER_SUF=$( if [ "${MAJ_VER}" -ge 2 ]; then echo "/v${MAJ_VER}"; else echo ""; fi )

# Check if the version is a release, having no "-xxx" suffix
if [ $(echo "${VERSION}" | grep "-") ]
    then echo "The current version is not a release" && exit 0
fi

# get old and new mod paths as a regexes
OLD_MOD_PATH=$(head -1 api/go.mod | awk '{print $2;}' | sed 's;\/;\\\/;g')
NEW_MOD_PATH=$( echo github.com/gravitational/teleport/api${MAJ_VER_SUF} |  sed 's;\/;\\\/;g')

# Check if the mod paths are the same, meaning the major version is the same
if [ "${OLD_MOD_PATH}" == "${NEW_MOD_PATH}" ]
    then echo "The api module path does not need to be updated" && exit 0
fi

# Update all instances of the mod path in .go and .proto fgsiles
find . -type f -iregex '.*\.\(go\|proto\|mod\)' -print0 \
    | xargs -0 sed -i -E "s/${OLD_MOD_PATH}/${NEW_MOD_PATH}/g"
    
# update go.mod require statements to use vX.0.0. 
sed -i "s/${NEW_MOD_PATH} v[0-9]\+.0.0/${NEW_MOD_PATH} v${MAJ_VER}.0.0/" go.mod examples/go-client/go.mod

# Update vendor to tidy modules and re-vendor api
make update-vendor

# Rebuild grpc files with updated proto files
make grpc

echo "Successfully updated api module path"
