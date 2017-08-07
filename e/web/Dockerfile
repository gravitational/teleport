FROM node:8.1.2-slim
RUN apt-get update && apt-get install -y python python-dev python-pip 

RUN mkdir -p /teleport/e/web
COPY package*.json /teleport/e/web/

VOLUME ["/teleport/web/"]
VOLUME ["/teleport/e/web/dist"]

WORKDIR /teleport/e/web

RUN npm install

COPY . /teleport/e/web

ARG UID
ARG GID

RUN chown $UID:$GID /teleport -R