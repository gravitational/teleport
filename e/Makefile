BUILDDIR ?= build
BINARIES=$(BUILDDIR)/tsh $(BUILDDIR)/teleport $(BUILDDIR)/tctl
LIBS = $(shell find lib -type f -name '*.go')

#
# Default target: builds all 3 executables and plaaces them in a current directory
#
.PHONY: all
all: $(BINARIES) 

$(BUILDDIR)/tctl: $(LIBS) tool/tctl/*.go
	go build -o $(BUILDDIR)/tctl -i $(BUILDFLAGS) ./tool/tctl

$(BUILDDIR)/teleport: $(LIBS) tool/teleport/*.go 
	go build -o $(BUILDDIR)/teleport -i $(BUILDFLAGS) ./tool/teleport

$(BUILDDIR)/tsh: $(LIBS) tool/tsh/*.go
	go build -o $(BUILDDIR)/tsh -i $(BUILDFLAGS) ./tool/tsh
