FROM node:8.1.2-slim

RUN mkdir -p /teleport/e/web
RUN mkdir -p /teleport/web

COPY package.json /teleport/e/web

WORKDIR /teleport/e/web

# RUN npm install

COPY ../../web/src /teleport/web
COPY web /teleport/e/web

#RUN npm install
#
#COPY . /app
#
#RUN groupadd builder --gid=$GID  -o ;\
#    useradd builder --uid=$UID --gid=$GID --create-home --shell=/bin/bash  ;\
#    chown builder /app -R
