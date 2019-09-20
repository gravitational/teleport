# Web Applications Enterprise Edition

This repository contains the proprietary code of Teleport and Gravity Web UI.

This repository should be initialized as a git submodule in the
Gravitational Web Apps repository. Building and developing enterprise
packages happens from there.

## How to build

Make sure that you clone the
[Gravitational Web Apps](https://github.com/gravitational/webapps)
mono-repository

```
$ git clone git@github.com:gravitational/webapps.git
```

Initialize the enterprise git sub-modules which creates
the `e` directory inside the `/packages` with the contents of this repository.

```
 $ cd webapps
 $ make init-submodules
```

```
/webapps  <-- open-source public Webapps mono-repository
  | / packages/
    |- /e  <-- this repo

```

### Build with docker


to build Teleport:
```
$ cd webapps
$ make teleport-e
```

to build Gravity:
```
$ cd webapps
$ make gravity-e
```

## Development

If `https://example.com:3080/web` is the URL of your cluster UI then:

to start your local Teleport development server
```
$ cd webapps
$ yarn start-teleport-e --target=https://example.com:3080/web
```

or to start your local Gravity development server
```
$ cd webapps
$ yarn start-gravity-e --target=https://example.com:3080/web
```
