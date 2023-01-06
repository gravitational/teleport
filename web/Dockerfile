FROM node:16.18-slim
RUN apt-get update && apt-get install git g++ make python3 tree -y

RUN mkdir -p web-apps
COPY yarn.lock web-apps/
COPY package.json web-apps/
COPY tsconfig.json web-apps/
# copy entire build package as it has required .bin files
COPY packages/build/ web-apps/packages/build/

# copy only package.json files
COPY packages/design/package.json web-apps/packages/design/
COPY packages/shared/package.json web-apps/packages/shared/
COPY packages/teleport/package.json web-apps/packages/teleport/
COPY packages/teleterm/package.json web-apps/packages/teleterm/

# copy enterprise package.json files if present
COPY README.md packages/webapps.e/telepor[t]/package.json web-apps/packages/webapps.e/teleport/

# download and install npm dependencies
WORKDIR web-apps
ARG YARN_FROZEN_LOCKFILE
RUN if [ -n "$YARN_FROZEN_LOCKFILE" ]; then \
      ./packages/build/scripts/yarn-install-frozen-lockfile.sh; \
    else \
      yarn install; \
    fi

# copy the rest of the files and run yarn build command
COPY  . .

ARG NPM_SCRIPT=nop
ARG OUTPUT
ARG CONNECT_TSH_BIN_PATH
ENV CONNECT_TSH_BIN_PATH=$CONNECT_TSH_BIN_PATH
# run npm script with optional --output-path parameter
RUN yarn $NPM_SCRIPT $([ -z $OUTPUT ] || echo --output-path=$OUTPUT)
