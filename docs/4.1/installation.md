# Installation

Teleport core service [`teleport`](cli-docs.md#teleport) and admin tools [`tctl`](cli-docs.md#tctl) has been designed to run on **Linux** and **Mac** operating systems.

The Teleport client [`tsh`](cli-docs.md#tsh) and Web UI are available for **Linux, Mac**
and **Windows** operating systems. Our examples install Teleport v4.1.0 but you can
install any version listed in our [Release History](https://gravitational.com/teleport/download/).

**Table of Contents**

[TOC]

## Checksums

Gravitational Teleport provides a checksum from the Downloads page.  This should
be used to verify the integrity of our binary.

![Teleport Checksum](./img/teleport-sha.png)

If you download Teleport via an automated system, you can programmatically
obtain the checksum  by adding `.sha256` to the binary. This is the method shown
in the installation examples.

```bash
$ export version=v4.1.0
$ export os=linux # 'darwin' 'linux' or 'windows'
$ export arch=amd64 # '386' 'arm' on linux or 'amd64' for all distros
$ curl https://get.gravitational.com/teleport-$version-$os-$arch-bin.tar.gz.sha256
# <checksum> <filename>
```

## Install Pre-built Binaries

!!! info "sudo"
    You may need to use `sudo` to run the installation commands.

### Linux

The following examples install the 64-bit version of Teleport binaries, but
32-bit (i386) and ARM binaries are also available. Check the [Latest
Release](https://gravitational.com/teleport/download/) page for the most
up-to-date information.

**Debian**
```bash
$ curl https://get.gravitational.com/teleport_4.1.0_amd64.deb.sha256
# <checksum> <filename>
$ curl -O https://get.gravitational.com/teleport_4.1.0_amd64.deb
$ sha256sum teleport_4.1.0_amd64.deb
# Verify that the checksums match
$ dpkg -i teleport_4.1.0_amd64.deb
$ which teleport
/usr/local/bin/teleport
```

**Tarball**
```bash
$ curl https://get.gravitational.com/teleport-v4.1.0-linux-amd64-bin.tar.gz.sha256
# <checksum> <filename>
$ curl -O https://get.gravitational.com/teleport-v4.1.0-linux-amd64-bin.tar.gz
$ shasum -a 256 teleport-v4.1.0-linux-amd64-bin.tar.gz
# Verify that the checksums match
$ tar -xzf teleport-v4.1.0-linux-amd64-bin.tar.gz
$ cd teleport
$ ./install
$ which teleport
/usr/local/bin/teleport
```

**RPM**
```bash
$ curl https://get.gravitational.com/teleport-4.1.0-1.x86_64.rpm.sha256
# <checksum> <filename>
$ curl -O https://get.gravitational.com/teleport-4.1.0-1.x86_64.rpm
$ sha256sum teleport-4.1.0-1.x86_64.rpm
# Verify that the checksums match
$ rpm -i teleport-4.1.0-1.x86_64.rpm
$ which teleport
/usr/local/bin/teleport
```

### MacOS

**Package**

* **Option 1:** Download the `.pkg` file via your web browser and double-click
  to run the Installer.
* **Option 2:** Install on the command line with `installer`
```bash
$ curl -O https://get.gravitational.com/teleport-4.1.0.pkg
$ sudo installer -pkg teleport-4.1.0.pkg -target / # Installs on Macintosh HD
Password:
installer: Package name is teleport-4.1.0
installer: Upgrading at base path /
installer: The upgrade was successful.
$ which teleport
/usr/local/bin/teleport
```

**Tarball**
```bash
$ curl https://get.gravitational.com/teleport-v4.1.0-darwin-amd64-bin.tar.gz.sha256
# <checksum> <filename>
$ curl -O https://get.gravitational.com/teleport-v4.1.0-darwin-amd64-bin.tar.gz
$ shasum -a 256 teleport-v4.1.0-darwin-amd64-bin.tar.gz
# Verify that the checksums match
$ tar -xzf teleport-v4.1.0-darwin-amd64-bin.tar.gz
$ cd teleport
$ ./install
$ which teleport
/usr/local/bin/teleport
```

### Windows (tsh client only)

As of version v4.1.0 we have `tsh` client binary available for Windows 64-bit
architecture - `teleport` and `tctl` are not supported.

```sh
> curl https://get.gravitational.com/teleport-v4.1.0-windows-amd64-bin.zip.sha256
# <checksum> <filename>
> curl -O https://get.gravitational.com/teleport-v4.1.0-windows-amd64-bin.zip
> echo %PATH% # Edit %PATH% if necessary
> certUtil -hashfile teleport-v4.1.0-windows-amd64-bin.zip SHA256
SHA256 hash of teleport-v4.1.0-windows-amd64-bin.zip:
# <checksum> <filename>
CertUtil: -hashfile command completed successfully.
# Verify that the checksums match
# Move `tsh` to your %PATH%
```

## Installing from Source

Gravitational Teleport is written in Go language. It requires **Golang v1.12.4**
or newer. Check [the repo
README](https://github.com/gravitational/teleport#building-teleport) for the
latest requirements.

### Install Go

If you don't already have Golang installed you can [see installation
instructions here](https://golang.org/doc/install). If you are new to Go there
are a few quick set up things to note.

- Go installs all dependencies _for all projects_ in a single directory
  determined by the `$GOPATH` variable. The default directory is
  `GOPATH=$HOME/go` but you can set it to any directory you wish.
- If you plan to use Golang for more than just this installation you may want to
  `echo "export GOPATH=$HOME/go" >> ~/.bashrc` (or your shell config).

### Build Teleport

```bash
# get the source & build:
$ mkdir -p $GOPATH/src/github.com/gravitational
$ cd $GOPATH/src/github.com/gravitational
$ git clone https://github.com/gravitational/teleport.git
$ cd teleport
# Make sure you have `zip` installed - the Makefile uses it
$ make full
# create the default data directory before running `teleport`
$ sudo mkdir -p /var/lib/teleport
$ sudo chown $USER /var/lib/teleport
```

If the build succeeds the binaries `teleport, tsh`, and `tctl` are now in the
directory `$GOPATH/src/github.com/gravitational/teleport/build`

<!--Notes on what to do if the build does not succeed, troubleshooting-->
