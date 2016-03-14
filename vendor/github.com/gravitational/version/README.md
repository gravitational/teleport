# version

`version` is a Go library for automatic build versioning. It attempts to simplify the mundane task of adding build version information to any Go package.

### Usage

Install the command line utility to generate the linker flags necessary for versioning from the cmd/linkflags:

```shell
go install github.com/gravitational/version/cmd/linkflags
```

Add the following configuration to your build script / Makefile
(assuming a bash script):

```bash
GO_LDFLAGS=$(linkflags -pkg=path/to/your/package)

# build with the linker flags:
go build -ldflags="${GO_LDFLAGS}"
```

To use, simply import the package and either obtain the version with `Get` or print the JSON-formatted version with `Print`:

```go
package main

import "github.com/gravitational/version"

func main() {
	version.Print()
}
```

If you have a custom vendoring solution, you might have this package stored under a different path than the default (`go get`).
In this case, you can override the default with a command line option (using [godep] as a vendoring solution):

```shell
MY_PACKAGE=github.com/my/package
MY_PACKAGE_PATH=$(pwd)
GO_LDFLAGS=$(linkflags -pkg=${MY_PACKAGE_PATH} -verpkg=${MY_PACKAGE}/Godeps/_workspace/src/github.com/gravitational/version)
```

The version part of the version information requires that you properly [tag] your packages:

```shell
$ git tag
v1.0.0-alpha.1
v1.0.0-beta.1
```

The build versioning scheme is a slight modification of the scheme from the [Kubernetes] project.
It consists of three parts:
  - a version string in [semver] format
  - git commit ID
  - git tree state (`clean` or `dirty`)

```go
type Info struct {
	Version      string `json:"version"`
	GitCommit    string `json:"gitCommit"`
	GitTreeState string `json:"gitTreeState"`
}
```


[//]: # (Footnots and references)

[Kubernetes]: <https://github.com/kubernetes/kubernetes>
[semver]: <http://semver.org>
[godep]: <https://github.com/tools/godep>
[tag]: <https://git-scm.com/book/en/v2/Git-Basics-Tagging>
