# Gravitational Web Applications and Packages

This mono-repository contains the source code for the
[Teleport](https://github.com/gravitational/teleport) and
[Gravity](https://github.com/gravitational/gravity) Web UI.

The code is organized in terms of independent yarn packages which reside in
the [packages directory](https://github.com/gravitational/webapps/tree/master/packages).

## Getting Started

You can make production builds locally or you can use Docker to do that.

### Setup Prettier on VSCode

1. Install plugin: https://github.com/prettier/prettier-vscode
1. Go to Command Palette: CMD/CTRL + SHIFT + P (or F1)
1. Type `open settings`
1. Select `Open Settings (JSON)`
1. Include the below snippet and save:

```js

// Autoformat on save
"editor.formatOnSave": false,

// Specify prettier configuration file
"prettier.configPath": ".prettierrc",

"[javascript]": {
    "editor.formatOnSave": true
},

"[javascriptreact]": {
    "editor.tabSize": 2,
    "editor.formatOnSave": true,
    "editor.defaultFormatter": "esbenp.prettier-vscode"
},

"[html]": {
    "editor.tabSize": 2,
    "editor.defaultFormatter": "esbenp.prettier-vscode"
}

```

### Local Build

Make sure that [you have yarn installed](https://yarnpkg.com/lang/en/docs/install/#debian-stable)
on your system since this monorepo uses the yarn package manager.

Then you need download and initialize these repository dependencies.

```
$ yarn install
```

To build Teleport Web UI

```
$ yarn build-teleport
```

To build Gravity Web UI

```
$ yarn build-gravity
```

The resulting output will be in `/packages/teleport/dist/` and `/packages/gravity/dist/`

### Docker Build

To build Teleport Web UI

```
$ make teleport
```

To build Gravity Web UI

```
$ make gravity
```

## Development

To avoid having to install a dedicated Teleport or Gravity cluster,
you can use a local development server which can proxy network requests
to an existing cluster.

For example, if `https://example.com:3080/web` is the URL of your cluster UI then:

to start your local Teleport development server

```
$ yarn start-teleport --target=https://example.com:3080/web
```

or to start your local Gravity development server

```
$ yarn start-gravity --target=https://example.com:3080/web
```

This service will serve your local javascript files and proxy network
requests to the given target.

> Keep in mind that you have to use a local user because social
> logins (google/github) are not supported by development server.

### Unit-Tests

We use [jest](https://jestjs.io/) as our testing framework.

To run all jest unit-tests:

```
$ yarn run test
```

To run jest in watch-mode

```
$ yarn run tdd
```

### Interactive Testing

We use [storybook](https://storybook.js.org/) for our interactive testing.
It allows us to browse our component library, view the different states of
each component, and interactively develop and test components.

To start a storybook:

```
$ yarn run storybook
```

This command will open a new browser window with storybook in it. There
you will see components from all packages so it makes it faster to work
and iterate on shared functionality.
