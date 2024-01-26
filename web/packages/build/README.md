# BUILD

This package contains build scripts used to build and develop
Gravitational packages. It was created specifically for Gravitational use.

## Content

`.eslintrc.js` a set of eslint rules used to validate JS code.

`.babelrc.js` a babel configuration file.

`./vite` contains the Vite configuration & plugins.

## Aliases

To make it easier for us to migrate legacy code and work on refactoring,
we introduced a few hardcoded aliases to shorten the package names in `imports`.

```
import '@gravitational/teleport/..' -> import 'teleport/...'
import '@gravitational/design/..' -> import 'design/...'
import '@gravitational/shared/..' -> import 'shared/...'
```
