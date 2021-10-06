# Web Applications Enterprise Edition

This repository contains the proprietary code of Teleport Web UI.

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

Initialize the enterprise git sub-modules.

```
 $ cd webapps
 $ make init-submodules
```

This will create `webapps.e` directory inside `webapps/packages`.

```
webapps
├── packages
│   ├── ...
│   ├── webapps.e   <-- this repo
│   └── README.md
└── ...

```

### Build with docker

Go to the root directory of `webapps` monorepo and run the following:

to build Teleport:

```
$ cd webapps
$ make teleport-e
```

## Development

If `https://example.com:3080/web` is the URL of your cluster UI then:

to start your local Teleport development server

```
$ cd webapps
$ yarn start-teleport-e --target=https://example.com:3080/web
```
