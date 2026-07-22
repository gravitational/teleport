---
authors: Zac Bergquist <zac@goteleport.com>
state: Implemented
---

# RFD 47 - Drop the vendor directory

## What

This RFD proposes that we stop committing the `vendor/` directory to source control.
This is a developer-facing change only and has no user-visible impact.

The `vendor/` directory contains the Go source code for the third party dependencies
required to build Teleport, and exists largely as a relic of early Go (pre-modules).

Prior to Go 1.11's introduction of
[modules](https://go.dev/blog/using-go-modules) (2018), vendoring was the
commonly-accepted convention for maintaining a set of project-specific
dependencies, ensuring that a vendored copy of a dependency is preferred over
a `$GOPATH`-wide version.

Teleport existed before Go modules, and has leveraged vendoring for several
years. When Teleport was converted to a Go module, we adopted the
[`go mod vendor`](https://go.dev/ref/mod#go-mod-vendor) tool as a way to leverage
modules while keeping the vendor directory and workflow in-tact.

## Why

At a high level, we propose eliminating the vendor directory for the following reasons:

1. It is no longer necessary, and the community at large is moving away from vendoring.
2. Dropping the vendor directory would result in a better developer experience.

## Details

### No Longer Necessary

In one of the [original proposals](https://research.swtch.com/vgo-module) for Go
modules, Russ Cox writes:

>    We want to eliminate vendor directories. They were introduced for
>    reproducibility and availability, but we now have better mechanisms.
>    Reproducibility is handled by proper versioning, and availability is handled by
>    caching proxies.

In short, vendoring was a solution for two common problems:

1. Reproducible builds
2. Availability of dependencies

We are already using Go modules, which also solve for these same problems.

Like vendoring, Go modules allow for a project to depend on a specific (or even
multiple) version of a dependency. Unlike vendoring, Go modules rely on a
shared, read-only module cache, which reduces unnecessary downloads and ensures
that multiple projects on the same workstation that share a common dependency
will build against the same codebase.

Vendoring also protects developers from scenarios where an upstream dependency
is deleted or otherwise made unavailable. Since a copy of the dependency is
maintained in Teleport's source code, Teleport will continue to build even if
upstream suddenly disappears. The solution to this problem in a Go modules world
is to use a [module proxy](https://go.dev/ref/mod#module-proxy) which caches and
serves modules. Additionally, building with a proxy is
[often faster](https://twitter.com/sajma/status/1155006281263923201?s=21), as
modules are served over HTTP instead of git, and dependencies can be resolved
quickly due to the go.mod file being served separately from the source code.

While it is possible to host and run [your own](https://docs.gomods.io) module
proxy, Teleport does not depend on any private Go modules (we have our own
solution for private source code), so we recommend leveraging the public proxy
that is [run by Google](https://proxy.golang.org) until we determine that
running our own proxy is warranted.

### Developer Experience

Committing the vendor directory has led to a number of minor issues, and
dropping it would result in an improved developer experience.

First, committing these changes results in repo bloat. Since go.mod and go.sum
already contain everything needed for a reproducible build, the teleport repo
ends up being larger than necessary. This causes clone operations to take longer,
increases the time taken for CI builds, etc.

Second, changes that bump dependencies are often large and difficult to review.
This is especially true if a change includes updates to dependencies and
teleport source, as reviewers may have to parse through large dependency diffs
in order to find the core teleport changes.

Additionally, our approach for the `api` module and vendoring requires a
particular workflow. We leverage a module replace directive, which allows us to
provide a versioned API module to external developers, while always building
against latest for official builds. Unfortunately, replace directives don't play
nice with vendoring, so we use an
[error-prone symbolic link](https://github.com/gravitational/teleport/blob/30effc1f08b6a699772ff22f79ebe756fe1a1e34/Makefile#L942-L952)
which is difficult to maintain in a cross-platform way, and has broken
[gopls integration](https://github.com/gravitational/teleport/blob/30effc1f08b6a699772ff22f79ebe756fe1a1e34/Makefile#L942-L952)
a common tool used in Go development environments.

Lastly, there is no guarantee that the code committed to vendor actually
reflects the contents of go.mod. The onus is on the developer to remember to run
`make update-vendor` and commit the results after making changes to
dependencies. This has created several cases of confusing build results amongst
the dev team in recent history.

In summary, dropping vendor would make go.mod/go.sum the single source of truth
for our dependencies, reduce the size of the teleport repo, make code changes
easier to review, and restore functionality with the latest version of `gopls`.

#### Testing Changes to Dependencies

With the vendoring approach, if a developer wants to test a change to one of our
dependencies, they can simply modify the file in the vendor directory and rebuild
the program.

If vendor is removed, this workflow becomes slightly more difficult, as the
module cache is read-only and may be shared across projects. In order to achieve
the same workflow without vendoring, developers must create a writeable copy of
the dependency on disk, and use a
[replace directive](https://go.dev/ref/mod#go-mod-file-replace) in order to
instruct the go tool to use the local copy instead of the version in the module
cache. [`gohack`](https://github.com/rogpeppe/gohack) is a tool that automates
this workflow:

```
# create a local copy of the module and set up a replace
# directive in go.mod
$ gohack get module/to/edit

# make some changes to the local copy
# build and test with the changes

# remove the replace directive
$ gohack undo module/to/edit
```

### Impact to CI

If the vendor directory is removed, each CI build will need to pull down
dependencies on first use. Should this become an issue, we can cache
the Go module cache in [Google Cloud Storage](https://cloud.google.com/build/docs/speeding-up-builds)
to speed up builds.

Also note that while Google Cloud Build performs a shallow checkout by default,
we are currently using `--unshallow` to fetch the entire history, so we will not
realize any benefits in terms of clone time. As of the time of this writing,
a full clone of the repository consumes 30-40 seconds of a 10+ minute build,
so there's unlikely to be a need to change this behavior.

### Security

The removal of the vendor directory does eliminate one workflow that is nice
from a security perspective and is worth discussing here. With vendoring, when a
dependency is updated, the Teleport commit diff shows the actual source code
changes to the dependency (or dependencies), allowing the developer to review
the changes for bugs or even malicious code. If the vendor directory is removed,
the developer would have to look at the release notes and code changes upstream
when updating dependencies in order to determine whether or not to apply the update.

While this "feature" sounds important in theory, in practice it is less useful.
The diff has to be just the right size to attract developer attention but not so
large as to disincentivize a review. In most cases, even with vendoring, a
developer simply sees a large set of dependency changes and glances over them
before moving on.

Supply chain security is as important as ever these days, and while our current
approach can (in some cases) make it easy for developers to audit changes, it
still requires a disciplined development team and careful attention to detail
and cannot be relied upon as the sole mechanism to prevent supply chain attacks.
In fact, Go modules arguably offer better supply chain security, as modules are
authenticated against an auditable checksum database. Some module proxies even
offer vulnerability scanning of the modules they serve.

Additionally, the Go project itself is making a number of improvements with
respect to supply chain security. Starting in Go 1.18, the `go` command will
embed additional version information in binaries, making it easier to generate a
software bill-of-materials. The Go team is also working on a Go vulnerability
[database](https://go.googlesource.com/proposal/+/master/design/draft-vulndb.md)
and tooling for reporting vulnerabilities in Go programs. Our experience mixing
modules and vendoring so far shows that the presence of a vendor directory in a
Go modules environment sometimes results in strange behavior, so removing the
vendor directory now can help us prepare to take advantage of these incoming
features.
