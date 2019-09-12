FROM node:12-slim
RUN apt-get update && apt-get install git -y

# copy only package.json files first
RUN mkdir -p web-apps
COPY yarn.lock web-apps/
COPY package.json web-apps/
# copy the build folder so yarn can install build/bin files in other packages
COPY packages/build/ web-apps/packages/build/
# copy just the package.json files to yarn install results
COPY packages/design/package.json web-apps/packages/design/
COPY packages/gravity/package.json web-apps/packages/gravity/
COPY packages/shared/package.json web-apps/packages/shared/
COPY packages/teleport/package.json web-apps/packages/teleport/
COPY packages/e/gravity/package.json web-apps/packages/e/gravity/
COPY packages/e/teleport/package.json web-apps/packages/e/teleport/
WORKDIR web-apps
RUN yarn install
COPY  . .
ARG NPM_SCRIPT
RUN yarn run $NPM_SCRIPT