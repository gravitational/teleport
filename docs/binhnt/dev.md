# Compile UI
    yarn 
    yarn build-ui-oss

=> Call 
@gravitational/teleport build


make docker-ui

make -C build.assets ui

# Run golang 
export CARGO_NET_GIT_FETCH_WITH_CLI=true 
make all 


