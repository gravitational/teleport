NAME = prehog
VERSION := $(shell git rev-parse HEAD)

DOCKER_REGISTRY = 146628656107.dkr.ecr.us-west-2.amazonaws.com/gravitational
DOCKER_IMAGE = ${DOCKER_REGISTRY}/${NAME}:${VERSION}

# likely linux/amd64 or linux/arm64, if set
DOCKER_PLATFORM ?=

.PHONY: prehog
prehog:
	CGO_ENABLED=0 go build -ldflags -w -trimpath -buildvcs=true -o ./build/prehog ./cmd/prehog

.PHONY: prehog-verbose
prehog-verbose: prehog
	shasum -a 256 ./build/prehog
	go version -m ./build/prehog

.PHONY: clean
clean:
	rm -rf ./build

.PHONY: lint
lint:
	./bin/golangci-lint run
	./bin/buf lint

.PHONY: gen
gen:
	rm -rf gen/proto
	./bin/buf generate

.PHONY: image
image:
	DOCKER_BUILDKIT=1 docker build --platform "${DOCKER_PLATFORM}" --tag "${DOCKER_IMAGE}" .

.PHONY: push
push:
	docker push "${DOCKER_IMAGE}"
