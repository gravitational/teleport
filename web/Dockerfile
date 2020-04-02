FROM node:12-slim
RUN apt-get update && apt-get install git -y
ENV TZ=America/New_York

RUN mkdir -p web-apps
COPY yarn.lock web-apps/
COPY package.json web-apps/
# copy entire build package as it has required .bin files
COPY packages/build/ web-apps/packages/build/

# copy only package.json files
COPY packages/design/package.json web-apps/packages/design/
COPY packages/gravity/package.json web-apps/packages/gravity/
COPY packages/shared/package.json web-apps/packages/shared/
COPY packages/teleport/package.json web-apps/packages/teleport/

# copy enterprise package.json files if present
COPY README.md packages/webapps.e/gravity/package.jso[n] web-apps/packages/webapps.e/gravity/
COPY README.md packages/webapps.e/teleport/package.jso[n] web-apps/packages/webapps.e/teleport/

# download and install npm dependencies
WORKDIR web-apps
RUN yarn install

# copy the rest of the files and run yarn build command
COPY  . .
ARG NPM_SCRIPT
RUN yarn run $NPM_SCRIPT
