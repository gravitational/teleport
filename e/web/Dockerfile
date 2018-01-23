FROM node:8.1.2-slim
RUN apt-get update && apt-get install -y python python-dev python-pip 

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