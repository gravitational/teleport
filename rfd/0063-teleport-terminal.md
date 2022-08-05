---
authors: Alexey Kontsevoy (alexey@goteleport.com)
state: draft
---

# RFD 0063 - Teleport Terminal

## What
Teleport Terminal is a desktop application that provides quick access to remote resources via Teleport.
This RFD defines the high-level architecture of Teleport Terminal.

## Details
There are two main components to Teleport Terminal:
1. teleterm
2. tsh (daemon mode)

### teleterm
`teleterm` is an [Electron](https://www.electronjs.org/) application that uses Chromium engine
for its tabbed UI and nodejs for OS-level operations.
Electron has been chosen because of cross-platform support and an ability to reuse most of existing Teleport Web UI components and design system.

#### UI
`teleterm` UI will have the following features:
1. Built-in fully featured terminal based on [xterm](https://xtermjs.org/) and [node-pty](https://github.com/microsoft/node-pty) to fork processes.
2. Tabbed layout where a tab can be an ssh session, rdp connection, or any other document.
3. Ability to add and access multiple Teleport clusters at the same time.
4. Command Palette for quick access and navigation.

### tsh daemon
`tsh daemon` is a `tsh` tool that runs as a service. A hidden flag launches `tsh` in a background process that exposes gRPC API over unix-socket (similarly to docker daemon).
This API is used by `teleterm` to call `tsh` internal methods to access Teleport clusters.
This makes `tsh` as `teleterm` backend service that stores information about clusters and retrieved certificates.

#### gRPC API
`tsh` gRPC API allows programmatic access to `tsh` functionality. This includes logging into a cluster, k8s, and creating a local proxy (alpn).
It uses unix-sockets as the primary communication channel. Unix Sockets are now broadly supported as Microsoft added Unix Sockets support to Windows (Windows 10 Version 1803).
On systems where Unix Sockets are not supported, `tsh` can establish a localhost TLS/TCP connection where TLS certificates are re-generated at start-time by `teleterm`.
Only unix-sockets are supported at this time.

### Security
`teleterm` runs under OS local user's privileges and does not require `root` access. It stores its state under user’s "app data" folder.
This includes UI state, settings, and temporary files such as unix-sockets.
Currently `tsh` profiles are also stored there as well.

`teleterm` minimizes the attack surface by delegating as much as possible to `tsh`. For example,
SSH to a server happens via executing a local `TELEPORT_CLUSTER=leafCluster tsh --proxy=rootCluster ssh login@server` command and piping it to the `pty`.

Database access happens via creating a local alpn proxy connection over tsh `API`. If MFA is required, `teleterm` receives a notification (over gRPC stream) before alpn proxy accepts a new connection request.

UI process (Electron renderer) runs in the context isolation mode with `nodejs` integration turned off. UI talks to tsh API and `node-pty` over [contextBridge](https://www.electronjs.org/docs/latest/api/context-bridge).
Even though UI does have access to local shells (via contextBridge -> node-pty), using `contextBridge` by default helps clear access boundaries between processes.

UI will ensure that general [security recommendations](https://www.electronjs.org/docs/latest/tutorial/security) are implemented.

## Packaging and installation
`teleterm` uses [Electron-Builder](https://github.com/electron-userland/electron-builder) that handles creation of packages for multiple platforms. `tsh` is packaged together with the
rest of the application and installed into the "app data" folder. After an installation, `teleterm` prompts a dialog asking a user to optionally register `tsh` and `teleterm` globally via symlinks.

Electron supports automatic updates. The updates happens via a publicly exposed service that Electron trusts. This service can be hosted by the
cloud team. This functionality currently is not implemented.


### Diagram
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
     |                        |        SNI/ALPN  | |
  +--+----------------------+ |         routing  | |
  |                         | |                  | |
  |     proxy connections   +-+                  | |
  |                         |                    | |
  +-------------------+-----+                    | |
                      ^                          | |
                      |                          | |
  +---------------+   | tls/tcp on localhost     | |
  |  tsh profiles |   |                          | |
  |    (files)    |   |                          v v
  |               |   |                   +------+-+-------------------+
  +-------^-------+   |                   |                            |
          |           +-------------------+           tsh              |
          +<------------------------------+         (daemon)           |
                                          |                            |
                                          +-------------+--------------+
 +--------+-----------------+                           ^
 |         Terminal         |                           |
 |    Electron Main Process |                           |        gRPC API
 +-----------+--------------+                           |     (domain socket)
             ^                                          |
             |                                          |
    IPC      |                                          |
 named pipes |                                          |
             v  Terminal UI (Electron Renderer Process) |
 +-----------+------------+---------------------------------------------+
 | -recently used         | root@node1 × | k8s_c  × | rdp_win2       ×  |
 |   root@node1           +---------------------------------------------+
 |   root@node2           |                                             |
 +------------------------+ ./                                          |
 | -clusters              | ../                                         |
 |  -cluster1             | assets/                                     |
 |    servers (20)        | babel.config.js                             |
 |    databases (12)      |                                             |
 |  +cluster2             |                                             |
 |  +cluster3             |                                             |
 +------------------------+---------------------------------------------+
```
