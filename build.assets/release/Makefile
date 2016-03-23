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

build: GOPATH := $(abspath .)
build: $(BUILDDIR)/tctl $(BUILDDIR)/teleport $(BUILDDIR)/tsh

$(BUILDDIR)/tctl $(BUILDDIR)/teleport $(BUILDDIR)/tsh:
	go build -o $@ -i $(BUILDFLAGS) $(PKGPATH)/tool/$(notdir $@)

.PHONY: install
install: build
	sudo cp -af $(BUILDDIR)/tctl $(BUILDDIR)/tsh $(BUILDDIR)/teleport      $(BINDIR)/
	sudo mkdir -p $(DATADIR)
	sudo cp -fr src/$(PKGPATH)/web/dist/* $(DATADIR)

.PHONY: clean
clean:
	rm -rf $(BUILDDIR)
