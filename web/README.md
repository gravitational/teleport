# Gravitational Web Applications and Packages

This mono-repository contains the source code for:

* the web UIs served by the `teleport` server
   * [`packages/teleport`](packages/teleport) for the OSS version
   * `packages/webapps.e` for the enterprise version
* the Electron app of [Teleport Connect](https://goteleport.com/connect/)
   * [`packages/teleterm`](packages/teleterm)

The code is organized in terms of independent yarn packages which reside in
the [packages directory](https://github.com/gravitational/webapps/tree/master/packages).

## Getting Started with Teleport Web UI

You can make production builds locally or you can use Docker to do that.

### Local Build

Make sure that [you have yarn installed](https://yarnpkg.com/lang/en/docs/install/#debian-stable)
on your system since this monorepo uses the yarn package manager.

Then you need download and initialize these repository dependencies.

```
$ yarn install
```

To build the Teleport open source version

```
$ yarn build-teleport-oss
```

The resulting output will be in the `/packages/{package-name}/dist/` folders respectively.

### Docker Build

To build the Teleport community version

```
$ make build-teleport-oss
```

## Getting Started with Teleport Connect

See [`README.md` in `packages/teleterm`](packages/teleterm).

## Development

### Web UI

To avoid having to install a dedicated Teleport cluster,
you can use a local development server which can proxy network requests
to an existing cluster.

For example, if `https://example.com:3080/web` is the URL of your cluster then:

To start your local Teleport development server

```
$ yarn start-teleport --target=https://example.com:3080/web
```

This service will serve your local javascript files and proxy network
requests to the given target.

> Keep in mind that you have to use a local user because social
> logins (google/github) are not supported by development server.

#### Caching

By default, Webpack will store a cache in `node_modules/.cache/webpack` during development. This
makes starting `webpack-dev-server` really quick after having ran it once, as it will re-use the
cache from the last time it was running.

If you want to change the location of the cache, you can set `WEBPACK_CACHE_DIRECTORY` to an 
absolute file path of the folder where you want to store Webpack's cache.

If you wish to disable the cache, you can set `WEBPACK_CACHE_DISABLED` to `yes`.

#### Source Maps

During development, Webpack will default to generating source maps using `eval-source-map`.
This can be overridden by setting the `WEBPACK_SOURCE_MAP` environment variable to one of the
[available values that Webpack offers](https://webpack.js.org/configuration/devtool/#devtool).

To turn them off, set `WEBPACK_SOURCE_MAP` to `none` -

```
$ WEBPACK_SOURCE_MAP=none yarn start-teleport --target=https://example.com:3080/web
```

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

### Setup Prettier on VSCode

1. Install plugin: https://github.com/prettier/prettier-vscode
1. Go to Command Palette: CMD/CTRL + SHIFT + P (or F1)
1. Type `open settings`
1. Select `Open Settings (JSON)`
1. Include the below snippet and save:

```js

    // Set the default
    "editor.formatOnSave": false,
    // absolute config path
    "prettier.configPath": ".prettierrc",
    // enable per-language
    "[html]": {
        "editor.formatOnSave": true,
        "editor.defaultFormatter": "esbenp.prettier-vscode"
    },
    "[javascript]": {
        "editor.formatOnSave": true,
        "editor.defaultFormatter": "esbenp.prettier-vscode"
    },
    "[javascriptreact]": {
        "editor.formatOnSave": true,
        "editor.defaultFormatter": "esbenp.prettier-vscode",
    },
    "[typescript]": {
        "editor.formatOnSave": true,
        "editor.defaultFormatter": "esbenp.prettier-vscode"
    },
    "[typescriptreact]": {
        "editor.formatOnSave": true,
        "editor.defaultFormatter": "esbenp.prettier-vscode",
    },
    "[json]": {
        "editor.formatOnSave": true,
        "editor.defaultFormatter": "vscode.json-language-features"
    },
    "[jsonc]": {
        "editor.formatOnSave": true,
        "editor.defaultFormatter": "vscode.json-language-features"
    },
    "[markdown]": {
        "editor.formatOnSave": true,
        "editor.defaultFormatter": "esbenp.prettier-vscode",
    },
    "editor.tabSize": 2,
```

### MFA Development

When developing MFA sections of the codebase, you may need to configure the `teleport.yaml` of your target teleport cluster to accept hardware keys registered over the local development setup. Webauthn can get tempermental if you try to use localhost as your `rp_id`, but you can get around this by using https://nip.io/. For example, if you want to configure optional `webauthn` mfa, you can set up your auth service like so:

```yaml
auth_service:
  authentication:
    type: local
    second_factor: optional
    webauthn:
      rp_id: proxy.0.0.0.0.nip.io

proxy_service:
  enabled: yes
  # setting public_addr is optional, useful if using different port e.g. 8080 instead of default 3080
  public_addr: ['proxy.0.0.0.0.nip.io']
```

Then start the dev server like `yarn start-teleport --target=https://proxy.127.0.0.1.nip.io:3080` and access it at https://proxy.127.0.0.1.nip.io:8080.

### Adding Packages/Dependencies

We use Yarn Workspaces to manage dependencies.

- [Introducing Workspaces](https://yarnpkg.com/blog/2017/08/02/introducing-workspaces)
- [Workspaces Documentation](https://yarnpkg.com/en/docs/workspaces)

The easiest way to add a package is to add a line to the workspace's `package.json` file and then run `yarn install` from
the root of this repository.

Keep in mind that there should only be a single `yarn.lock` in this repository, here at the top level. If you add packages
via `yarn workspace <workspace-name> add <package-name>`, it will create a `packages/<package-name>/yarn.lock` file, which should not be checked in.
