# Gravitational Web Applications and Packages

This directory contains the source code for:

- the web UIs served by the `teleport` server
  - [`packages/teleport`](packages/teleport)
- the Electron app of [Teleport Connect](https://goteleport.com/connect/)
  - [`packages/teleterm`](packages/teleterm)

The code is organized in terms of independent yarn packages which reside in
the [packages directory](packages).

## Getting Started with Teleport Web UI

You can make production builds locally or you can use Docker to do that.

### Local Build

Make sure that you have [Yarn 1](https://classic.yarnpkg.com/en/docs/install/) installed. The
Node.js version should match the one reported by executing `make -C build.assets print-node-version`
from the root directory.

Then you need to download and initialize JavaScript dependencies.

```
yarn install
```

To build the Teleport open source version

```
yarn build-ui-oss
```

The resulting output will be in the `webassets` folder.

### Docker Build

To build the Teleport community version

```
make docker-ui
```

## Getting Started with Teleport Connect

See [`README.md` in `packages/teleterm`](packages/teleterm#readme).

## Development

### Local HTTPS

To run `vite` for either Teleport or Teleport enterprise, you'll need to generate local
self-signed certificates. The recommended way of doing this is via [mkcert](https://github.com/FiloSottile/mkcert).

You can install mkcert via

```
brew install mkcert
```

After you've done this, run:

```
mkcert -install
```

This will generate a root CA on your machine and automatically trust it (you'll be prompted for your password).

Once you've generated a root CA, you'll need to generate a certificate for local usage.

Run the following from the `web/` directory, replacing `localhost` if you're using a different hostname.

```
mkdir -p certs && mkcert -cert-file certs/server.crt -key-file certs/server.key localhost "*.localhost"
```

(Note: the `certs/` directory in this repo is ignored by git, so you can place your certificate/keys
in there without having to worry that they'll end up in a commit.)

#### Certificates in an alternative location

If you already have local certificates, you can set the environment variables:

- `VITE_HTTPS_CERT` **(required)** - absolute path to the certificate
- `VITE_HTTPS_KEY` **(required)** - absolute path to the key

You can set these in your `~/.zshrc`, `~/.bashrc`, etc.

```
export VITE_HTTPS_CERT=/Users/you/certs/server.crt
export VITE_HTTPS_KEY=/Users/you/certs/server.key
```

### Web UI

To avoid having to install a dedicated Teleport cluster,
you can use a local development server which can proxy network requests
to an existing cluster.

For example, if `https://example.com:3080` is the URL of your cluster then:

To start your local Teleport development server

```
PROXY_TARGET=example.com:3080 yarn start-teleport
```

If you're running a local cluster at `https://localhost:3080`, you can just run

```
yarn start-teleport
```

This service will serve your local javascript files and proxy network
requests to the given target.

> Keep in mind that you have to use a local user because social
> logins (google/github) are not supported by development server.

### WASM

The web UI includes a WASM module built from a Rust codebase located in `packages/teleport/src/ironrdp`.
It is built with the help of [wasm-pack](https://github.com/rustwasm/wasm-pack).

Running `yarn build-wasm` builds the WASM binary as well as the appropriate Javascript/Typescript
bindings and types in `web/packages/teleport/src/ironrdp/pkg`.

### Unit-Tests

We use [jest](https://jestjs.io/) as our testing framework.

To run all jest unit-tests:

```
yarn run test
```

To run jest in watch-mode

```
yarn run tdd
```

### Interactive Testing

We use [storybook](https://storybook.js.org/) for our interactive testing.
It allows us to browse our component library, view the different states of
each component, and interactively develop and test components.

To start a storybook:

```
yarn run storybook
```

This command will open a new browser window with storybook in it. There
you will see components from all packages so it makes it faster to work
and iterate on shared functionality.

### Browser compatibility

We are targeting last 2 versions of all major browsers. To quickly find out which ones exactly, use the following command:

```
yarn browserslist 'last 2 chrome version, last 2 edge version, last 2 firefox version, last 2 safari version'
```

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
      rp_id: proxy.127.0.0.1.nip.io

proxy_service:
  enabled: yes
  # setting public_addr is optional, useful if using different port e.g. 8080 instead of default 3080
  public_addr: ['proxy.127.0.0.1.nip.io']
```

Then start the dev server like `PROXY_TARGET=https://proxy.127.0.0.1.nip.io:3080 yarn start-teleport` and access it at https://proxy.127.0.0.1.nip.io:8080.

### Adding Packages/Dependencies

We use Yarn Workspaces to manage dependencies.

- [Introducing Workspaces](https://yarnpkg.com/blog/2017/08/02/introducing-workspaces)
- [Workspaces Documentation](https://yarnpkg.com/en/docs/workspaces)

The easiest way to add a package is to add a line to the workspace's `package.json` file and then run `yarn install` from
the root of this repository.

Keep in mind that there should only be a single `yarn.lock` in this repository, here at the top level. If you add packages
via `yarn workspace <workspace-name> add <package-name>`, it will create a `packages/<package-name>/yarn.lock` file, which should not be checked in.

### Adding an Audit Event

When a new event is added to Teleport, the web UI has to be updated to display it correctly:

1. Add a new entry to [`eventCodes`](https://github.com/gravitational/webapps/blob/8a0201667f045be7a46606189a6deccdaee2fe1f/packages/teleport/src/services/audit/types.ts).
2. Add a new entry to [`RawEvents`](https://github.com/gravitational/webapps/blob/8a0201667f045be7a46606189a6deccdaee2fe1f/packages/teleport/src/services/audit/types.ts) using the event you just created as the key. The fields should match the fields of the metadata fields on `events.proto` on Teleport repository.
3. Add a new entry in [Formatters](https://github.com/gravitational/webapps/blob/8a0201667f045be7a46606189a6deccdaee2fe1f/packages/teleport/src/services/audit/makeEvent.ts) to format the event on the events table. The `format` function will receive the event you added to `RawEvents` as parameter.
4. Define an icon to the event on [`EventIconMap`](https://github.com/gravitational/webapps/blob/8a0201667f045be7a46606189a6deccdaee2fe1f/packages/teleport/src/Audit/EventList/EventTypeCell.tsx).
5. Add an entry to the [`events`](https://github.com/gravitational/webapps/blob/8a0201667f045be7a46606189a6deccdaee2fe1f/packages/teleport/src/Audit/fixtures/index.ts) array so it will show up on the [`AllEvents` story](https://github.com/gravitational/webapps/blob/8a0201667f045be7a46606189a6deccdaee2fe1f/packages/teleport/src/Audit/Audit.story.tsx)
6. Check fixture is rendered in storybook, then update snapshot for `Audit.story.test.tsx` using `yarn test-update-snapshot`.

You can see an example in [this pr](https://github.com/gravitational/webapps/pull/561).
