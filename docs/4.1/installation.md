# Installing

To install, download the official binaries from the [Teleport Downloads](https://gravitational.com/teleport/download/)

Previous versions are documented in [Release History](https://gravitational.com/teleport/releases/)

## Installing from Source

Gravitational Teleport is written in Go language. It requires Golang v1.8.3 or
newer.

If you don't already have Golang installed you can [install it here](https://golang.org/doc/install). If you are new to Go there are a few quick set up things to note.

- Go installs all dependencies in a single directory determined by the `$GOPATH` variable. The default directory is `GOPATH=$HOME/go` but you can set it to any directory you wish.
- If you plan to use Golang for more than just this installation you may want to `echo "export GOPATH=$HOME/go" >> ~/.bashrc` (or your shell config).

```bash
# get the source & build:
$ mkdir -p $GOPATH/src/github.com/gravitational
$ cd $GOPATH/src/github.com/gravitational
$ git clone https://github.com/gravitational/teleport.git
$ cd teleport
# Make sure you have `zip` installed because the Makefile uses it
$ make full
# create the default data directory before starting:
$ sudo mkdir -p /var/lib/teleport
```

## Linux

The following commands will install v4.1.0 on the CLI

```bash
$ export version=v4.1.0
$ export os=linux # 'darwin' 'linux' or 'windows'
$ export arch=amd64 # '386' 'arm' on linux or 'amd64' for all distros
# Automated way to retrieve the checksum, just append .sha256
$ curl https://get.gravitational.com/teleport-$version-$os-$arch-bin.tar.gz.sha256
[Checksum output]
$ curl -O https://get.gravitational.com/teleport-$version-$os-$arch-bin.tar.gz
$ shasum -a 256 teleport-$version-$os-$arch-bin.tar.gz 
# ensure the checksum matches the sha256 checksum on the download page!
$ tar -xzf teleport-$version-$os-$arch-bin.tar.gz
$ cd teleport
# this copies the binaries to /usr/local/bin
$ sudo ./install
$ which teleport
/usr/local/bin/teleport
```

You can also get the binaries with rpm
```bash
$ export version=4.1.0 # note: no 'v' prefix
$ export arch=x86_64 # 'i386' also availables
$ curl -O https://get.gravitational.com/teleport-$version-1.$arch.rpm
# this copies the binaries to /usr/local/bin
$ sudo rpm -i teleport-$version-1.$arch.rpm
$ which teleport
/usr/local/bin/teleport
```

### Teleport Checksum

Gravitational Teleport provides a checksum from the Downloads page.  This can be used to 
verify the integrity of our binary. 

![Teleport Checksum](img/teleport-sha.png)

**Checking Checksum on Automated Systems**

If you download Teleport via an automated system, you can programmatically obtain the checksum 
by adding `.sha256` to the binary. 

```bash
$ curl https://get.gravitational.com/teleport-$version-$os-$arch-bin.tar.gz.sha256
[Checksum output]
```

**Checking the Checksum**

For Linux or Mac OS
```bash
$ shasum -a 256 teleport-$version-$os-$arch-bin.tar.gz 
```

For Windows
```
certUtil -hashfile teleport-$version-windows-amd64-bin.zip SHA256
```

## Mac

TODO

### Teleport Checksum

**Checking the Checksum**

For Linux or Mac OS
```bash
$ shasum -a 256 teleport-$version-$os-$arch-bin.tar.gz 
```



## Windows

TODO

### Teleport Checksum

**Checking the Checksum**

For Windows
```
certUtil -hashfile teleport-$version-windows-amd64-bin.zip SHA256
```