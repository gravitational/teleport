OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
BUILDDIR ?= build
CGOFLAG ?= CGO_ENABLED=0
ADDFLAGS ?=

PLUGIN_NAME = buf-plugin-ensure-paginated
SUBJECT = .


.PHONY: build
build: $(BUILDDIR)/$(PLUGIN_NAME)
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -o $(BUILDDIR)/$(PLUGIN_NAME) $(BUILDFLAGS) $(SUBJECT)

.PHONY: clean
clean:
	go clean

.PHONY: install
install: build
	go install $(SUBJECT)

.PHONY: test
test: FLAGS ?= -race -shuffle on
test:
	go test -cover -json $(FLAGS) $(ADDFLAGS) $(SUBJECT)

.PHONY: lint
lint:
	golangci-lint run -c ../../../.golangci.yml
