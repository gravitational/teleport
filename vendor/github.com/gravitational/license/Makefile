#
# test runs tests for all packages
#
.PHONY: test
test:
	go test -v -test.parallel=0 -race ./...

#
# build builds all packages
#
.PHONY: build
build:
	go build ./...
