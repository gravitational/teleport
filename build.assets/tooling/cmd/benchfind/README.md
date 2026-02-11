# benchfind

A CLI utility to discover modules with Benchmarks without compilation/linking. 

Example useage:

```sh
# Find all Benchmark packages in the current dir:
benchfind

# Find all Benchmark packages matching ./lib/...:
benchfind ./lib/...

# Find all Benchmark packages matching given tags:
benchfind --tags=integration,linux

# Find all Benchmark packages and exclude a given package:
benchfind -e github.com/gravitational/teleport/foo/bar
```

The reason this is useful in a large codebase is that to run `go test -bench ./...` would require the linking of every test target binary. This can be very computationally heavy. Instead `benchfind` relies on the golang AST to find the relevant packages.

Example use:
```sh
go test -run ^$$ -bench .  $(benchfind)
```