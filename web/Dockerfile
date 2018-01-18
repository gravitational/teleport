FROM node:8.1.2-slim

RUN apt-get update && apt-get install -y python python-dev python-pip 

# prepare to install only package.json dependencies 
RUN mkdir -p /app
COPY package*.json /app/

# copy the rest of the app files
WORKDIR /app
RUN npm install
COPY . /app

# and build
RUN npm run build