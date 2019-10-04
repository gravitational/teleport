# Production Guide

TODO Build off Quickstart, but include many more details. 

Include security considerations.

<!--## Installing and Starting

This guide installs teleport v4.1.0 on the CLI. Previous version and are documented in [Release History](https://gravitational.com/teleport/releases/)

You can download pre-built binaries from our [Downloads](https://gravitational.com/teleport/download/) page.
or you can [build it from source](https://gravitational.com/teleport/docs/admin-guide/#installing-from-source).

You can also download `.deb`, `.rpm`, and `.pkg` files from [Downloads](https://gravitational.com/teleport/download/)

```bash
$ export version=v4.1.0
$ export os=linux # 'darwin' 'linux' or 'windows'
$ export arch=amd64 # '386' 'arm' on linux or 'amd64' for all distros
$ curl -O https://get.gravitational.com/teleport-$version-$os-$arch-bin.tar.gz
$ shasum -a 256 teleport-$version-$os-$arch-bin.tar.gz 
# ensure the checksum matches the value on the download page!
$ tar -xzf teleport-$version-$os-$arch-bin.tar.gz
$ cd teleport
$ sudo ./install
```

This will copy Teleport binaries to `/usr/local/bin`.

Let's start Teleport. First, create a directory for Teleport
to keep its data. By default it's `/var/lib/teleport`. Then start `teleport` daemon:

```bash
$ sudo teleport start
```

!!! danger "WARNING":
    Teleport stores data in `/var/lib/teleport`. Make sure that regular/non-admin users do not
    have access to this folder on the Auth server.


If you are logged in as `root` you may want to create a new OS-level user first. On linux create a new user called `<username>` with the following commands: 
```bash
$ adduser <username>
$ su <username>
```

Security considerations on installing tctl under root or not-->
