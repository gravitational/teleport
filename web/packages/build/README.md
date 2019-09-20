# BUILD

This package contains build scripts used to build and develop
Gravitational packages. It was created specifically for Gravitational use.

## Content

`devServer.js` starts a custom webpack development server and proxies
network requests to a given target.

> Gravitational API paths are hardcoded inside this file

`devServerUtils.js` adds custom logic to the proxy handlers. It inserts
bearer and CSRF tokens taken from the original index.html file into your
local version, so you can successfully authenticate against a targeted server.

`.eslintrc.js` a set of eslint rules used to validate JS code.

`.babelrc.js` a babel configuration file.

`./webpack` contains webpack configs for dev/production builds.

`./bin` a set of build scripts which get copied to the package `node_modules` folder.

## How to use it

Add `@gravitational/build` to your package.json file.

```
"devDependencies": {
    "@gravitational/build": "^1.0.0",
  },
```

Create `./src` directory and `boot.js` file. This file is the entry point of your
application and is used by default in webpack config.

```
    entry: {
      app: ['./src/boot.js'],
    },
```

Then you can run

```
$ yarn gravity-build
```

## Custom webpack config

If you want to use your own `webpack.config.js` file and override the defaults:

```
const defaultCfg = require('@gravitational/build/webpack/webpack.prod.config');

defaultCfg.entry = {
  app: ['./src/myentry.js'],
},

module.exports = defaultCfg;
```

Then run:

```
$ yarn gravity-build --config webpack.config.js
```

## Aliases

To make it easier for us to migrate legacy code and work on refactoring,
we introduced a few hardcoded aliases to shorten the package names in `imports`.

```
import '@gravitational/teleport/..' -> import 'teleport/...'
import '@gravitational/gravity/..' -> import 'gravity/...'
import '@gravitational/design/..' -> import 'design/...'
import '@gravitational/shared/..' -> import 'shared/...'
```