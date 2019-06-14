FROM node:8.11.2-slim

# Fixes jessie repository
RUN sed -i '/jessie-updates/d' /etc/apt/sources.list

RUN apt-get update && apt-get install git -y && apt-get install -y python python-dev python-pip

RUN npm i npm@6.4.1 -g

RUN mkdir -p /teleport/e/web
COPY package*.json /teleport/e/web/

WORKDIR /teleport/e/web

# install npm modules
RUN npm install

# copy teleport dependencies
COPY telebase /teleport/e/web
COPY telebase-examples /teleport/e/web

# copy E files
COPY . /teleport/e/web

# and build E
RUN npm run build