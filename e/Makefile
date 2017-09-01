BUILDDIR ?= build
BUILDFLAGS ?= $(ADDFLAGS) -ldflags '-w -s'

LIBS = $(shell find lib -type f -name '*.go')
OSSLIBS = $(shell find ../lib -type f -name '*.go')

TCTLSRC = $(shell find tool/tctl -type f -name '*.go')
TELEPORTSRC = $(shell find tool/teleport -type f -name '*.go')

.PHONY: all
all: 
	go install $(BUILDFLAGS) ../lib/...  ./lib/...
	make -j2 -s $(BUILDDIR)/tctl $(BUILDDIR)/teleport

$(BUILDDIR)/tctl: $(LIBS) $(OSSLIBS) $(TCTLSRC)
	go build -o $(BUILDDIR)/tctl -i $(BUILDFLAGS) ./tool/tctl

$(BUILDDIR)/teleport: $(LIBS) $(OSSLIBS) $(TELEPORTSRC)
	go build -o $(BUILDDIR)/teleport -i $(BUILDFLAGS) ./tool/teleport


.PHONY: clean
clean:
	rm -rf $(BUILDDIR)
