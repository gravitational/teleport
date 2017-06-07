FROM node:5-slim

ARG UID
ARG GID

VOLUME ["/tmp/copy"]

RUN mkdir -p /app

COPY package.json /app/

WORKDIR /app

COPY telebase/web/ /app/telebase/web

RUN npm install

COPY . /app

RUN groupadd builder --gid=$GID  -o ;\
    useradd builder --uid=$UID --gid=$GID --create-home --shell=/bin/bash  ;\
    chown builder /app -R
