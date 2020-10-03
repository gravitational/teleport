# Fuzzers

The fuzz package holds fuzzers for the Teleport library.

All fuzzers are stored in fuzz.go and are implemented using [go-fuzz](https://github.com/dvyukov/go-fuzz)

To run a fuzzer locally, follow these steps:
1. go get github.com/gravitational/teleport
2. go get -u github.com/dvyukov/go-fuzz/go-fuzz
3. go get -u github.com/dvyukov/go-fuzz/go-fuzz-build
4. cd into dir of fuzz.go
5. $GOPATH/bin/go-fuzz-build
6. $GOPATH/bin/go-fuzz
