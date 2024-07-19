# BUILD

This package contains build scripts used to build and develop
Gravitational packages. It was created specifically for Gravitational use.

## Content

`devserver` is Gravitational local development server based on the `webpack-dev-server` package.
It works as a proxy to Gravitational clusters where it redirects API requests to the given target
while serving your web assets locally. This proxy knows how to handle CSRF and bearer tokens
embedded in index.html file.

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

## Aliases

To make it easier for us to migrate legacy code and work on refactoring,
we introduced a few hardcoded aliases to shorten the package names in `imports`.

```
import '@gravitational/teleport/..' -> import 'teleport/...'
import '@gravitational/design/..' -> import 'design/...'
import '@gravitational/shared/..' -> import 'shared/...'
```
