# Gravitational Force Web UI

This package contains the source code of Force Web UI

## Build

```
$ yarn build
```

The dist files will be created in the `/packages/force/dist/` directory


## Development

If `https://example.com:3080/web` is your server URL then you can
start local development server:

```
$ yarn start --target=https://example.com:3080/web
```

### Typescript

To run a type check

```
$ yarn type-check
```

To run a type check in watch mode

```
$ yarn type-watch
```