# Admin Guide

### Building

Gravitational Teleport is written in Go and requires Golang v1.5 or newer. If you have Go
already installed, building is easy:

```bash
> git clone https://github.com/gravitational/teleport && cd teleport
> make
```

If you do not have Go but you have Docker installed and running, you can build Teleport
this way:

```bash
> git clone https://github.com/gravitational/teleport
> make -C build.assets
```

### Installing

TBD

- Configuration
- Adding users to the cluster
- Adding nodes to the cluster
- Controlling access
