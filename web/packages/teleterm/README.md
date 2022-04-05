## Teleport Terminal

Teleport Terminal (teleterm) is a desktop application that allows easy access to Teleport resources.

### Building and Packaging

Teleport Terminal consists of two main components: the `tsh` tool and the Electron app. Our build
scripts assume that the `webapps` repo and the `teleport` repo are in the same folder.

To get started, first we need to build `tsh` that resides in the `teleport` repo.


Prepare Teleport repo:

```bash
## Clone Teleport repo
$ git clone https://github.com/gravitational/teleport.git
$ cd teleport
## Build tsh binary
$ make build/tsh
```

The build output can be found in the `/teleport/build` directory. The tsh binary will be packed
together with the Electron app.

Prepare Webapps repo
1. Make sure that your node version is v16 (current tls) https://nodejs.org/en/about/releases/
2. Clone and build `webapps` repository
```bash
$ git clone https://github.com/gravitational/webapps.git
$ cd webapps
$ yarn install
$ yarn build-term
$ yarn package-term
```

The installable file can be found in `/webapps/packages/teleterm/build/release/`

### Development

**Make sure to run `yarn build-term` first** (as described above) before attempting to launch the
app in the development mode. That's because Electron is running its own version of Node. That
command will fetch native packages that were built for that specific version of Node.

To launch `teleterm` in the development mode:

```sh
$ cd webapps

## TELETERM_TSH_PATH is the environment variable that points to local tsh binary
$ TELETERM_TSH_PATH=$PWD/../teleport/build/tsh yarn start-term
```

For quick restarts, that restarts all processes and `tsh` daemon, press `F6`.

### Tips

#### gRPC protobuf files

Rebulding them is needed only if you change any of the files in `/teleport/lib/teleterm/api/proto/`
dir.

1. To rebuild and update `tsh` grpc proto files

```sh
$ cd teleport
$ make grpc-teleterm
```

Resulting files both `nodejs` and `golang` can be found in `/teleport/lib/teleterm/api/protogen/` directory.

```pro
lib/teleterm/api/protogen/
├── golang
│   └── v1
│       ├── auth_challenge.pb.go
│       ├── auth_settings.pb.go
│       ├── ...
│       └── ...
└── js
    └── v1
        ├── service_grpc_pb.js
        ├── service_pb.d.ts
        └── ...
```

2. Update `nodejs` files by copying them to the `/webapps/packages/teleterm/src/services/tshd/` location

```sh
$ cd teleport
$ rm -rf ./../webapps/packages/teleterm/src/services/tshd/v1/ && cp -R lib/teleterm/api/protogen/js/v1 ./../webapps/packages/teleterm/src/services/tshd/v1
```

### Architecture diagram
```pro
                                                  +------------+
                                                  |            |
                                          +-------+---------+  |
                                          |                 |  |
                                          |    teleport     +--+
                                          |     clusters    |
                                          |                 |
                                          +------+-+--------+
                                                 ^ ^           External Network
+------------------------------------------------|-|---------------------+
                                                 | |           Host OS
           Clients (psql)                        | |
              |                                  | |
              v                                  | |
     +--------+---------------+                  | |
     |                        |        SNI/ALPN  | | GRPC
  +--+----------------------+ |         routing  | |
  |                         | |                  | |
  |     local proxies       +-+                  | |
  |                         |                    | |
  +-------------------+-----+                    | |
                      ^                          | |
                      |                          | |
  +---------------+   | tls/tcp on localhost     | |
  |    local      |   |                          | |
  | user profile  |   |                          v v
  |   (files)     |   |                   +------+-+-------------------+
  +-------^-------+   |                   |                            |
          ^           +-------------------+         tsh daemon         |
          |                               |          (golang)          |
          +<------------------------------+                            |
                                          +-------------+--------------+
 +--------+-----------------+                           ^
 |         Terminal         |                           |
 |    Electron Main Process |                           |    GRPC API
 +-----------+--------------+                           | (domain socket)
             ^                                          |
             |                                          |
    IPC      |                                          |
 named pipes |                                          |
             v  Terminal UI (Electron Renderer Process) |
 +-----------+------------+---------------------------------------------+
 | -gateways              | root@node1 × | k8s_c  × | rdp_win2 ×  |     |
 |   https://localhost:22 +---------------------------------------------+
 |   https://localhost:21 |                                             |
 +------------------------+ ./                                          |
 | -clusters              | ../                                         |
 |  -cluster1             | assets/                                     |
 |   +servers             | babel.config.js                             |
 |     node1              | build/                                      |
 |     node2              | src/                                        |
 |   -dbs                 |                                             |
 |    mysql+prod          |                                             |
 |    mysql+test          |                                             |
 |  +cluster2             |                                             |
 |  +cluster3             |                                             |
 +------------------------+---------------------------------------------+
```

