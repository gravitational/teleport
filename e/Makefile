BUILDDIR ?= build
BINARIES=$(BUILDDIR)/teleport $(BUILDDIR)/tctl
LIBS = $(shell find lib -type f -name '*.go')

#
# Default target: builds 'tctl' and 'teleport' and plaaces them into ./build/
#
.PHONY: all
all: $(BINARIES) 

$(BUILDDIR)/tctl: $(LIBS) tool/tctl/*.go
	go build -o $(BUILDDIR)/tctl -i $(BUILDFLAGS) ./tool/tctl

$(BUILDDIR)/teleport: $(LIBS) tool/teleport/*.go 
	go build -o $(BUILDDIR)/teleport -i $(BUILDFLAGS) ./tool/teleport

.PHONY: clean
clean:
	rm -rf $(BUILDDIR)
