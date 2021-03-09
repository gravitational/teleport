# Teleport Docs

Teleport docs are built using [next.js](https://github.com/gravitational/next) and hosted using vercel.com

## Writing guide

[Please read the guide on docs](https://goteleport.com/teleport/docs/docs/)

## To Publish New Version

Publishing is happening automatically once you merge PR to `master` branch.

## Running Locally

We recommend using node directly to run local version.
It is much faster and has less problems than docker based version.

Local development is done from `https://github.com/gravitational/next` repository.

### Getting Started

**Requirements**

 Node > 14.x
 Linux or Mac
 
**Installing**

```bash
# Install NVM
# https://github.com/nvm-sh/nvm#installing-and-updating
$ curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.37.2/install.sh | bash
# Install node
$ nvm install node --lts
$ nvm use system
$ npm install --global yarn
$ git clone https://github.com/gravitational/next.git 
$ cd next
$ yarn install
$ git submodule init
$ git submodule update
$ yarn dev
# go to http://localhost:3000/teleport/docs
```

**Running your local version**

To checkout version of Teleport's docs:

```
$ cd next/content/teleport
$ git checkout your branch
```
