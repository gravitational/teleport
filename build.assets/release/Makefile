# these are standard autotools variables, don't change them please
BUILDDIR ?= build
BUILDFLAGS ?= %BUILDFLAGS%
BINDIR ?= /usr/bin
DATADIR ?= /usr/share/teleport
PKGPATH := github.com/gravitational/teleport
GO15VENDOREXPERIMENT := 1
export

#
# Default target: builds all 3 executables and plaaces them in a current directory
#
.PHONY: all
all: build

.PHONY: build
build: GOPATH := $(abspath .)
build:
	go build -o $(BUILDDIR)/tctl -i $(BUILDFLAGS) $(PKGPATH)/tool/tctl
	go build -o $(BUILDDIR)/teleport -i $(BUILDFLAGS) $(PKGPATH)/tool/teleport
	go build -o $(BUILDDIR)/tsh -i $(BUILDFLAGS) $(PKGPATH)/tool/tsh

.PHONY: install
install: build
	sudo cp -f $(BUILDDIR)/tctl      $(BINDIR)/
	sudo cp -f $(BUILDDIR)/tsh       $(BINDIR)/
	sudo cp -f $(BUILDDIR)/teleport  $(BINDIR)/
	sudo mkdir -p $(DATADIR)
	sudo cp -fr web/dist/* $(DATADIR)

.PHONY: clean
clean:
	rm -rf $(BUILDDIR)
