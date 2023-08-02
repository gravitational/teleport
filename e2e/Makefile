ARCH=$(shell go env GOARCH)
OS=$(shell go env GOOS)

.PHONY: test clean stop build
test:
	mkdir -p build
# Build binaries if they don't exist
ifneq ("$(wildcard $(../build/teleport))","")
	make build-binaries
endif
	docker compose up --abort-on-container-exit --exit-code-from e2e

clean:
	docker volume rm  e2e_teleport-config e2e_teleport-data

stop:
	docker compose down

build:
	docker compose build --build-arg BUILDARCH=$(ARCH)
	docker compose create

build-binaries:
# Use Docker to build binaries on MacOS as the testsuite runs in a Linux container
ifeq ($(OS),"darwin")
 	ARCH=$(ARCH) make -C ../build.assets/ build-binaries
else ifeq ($(OS),"linux")
	make -C ../build.assets/ full
else
	@echo "Unsupported OS: $(OS)"
	@exit 1
endif