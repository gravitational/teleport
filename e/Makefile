# This makefile usually runs as a sub-make by ../Makefile
# But running regular 'make' and 'make clean' works.
BUILDDIR ?= build
BUILDFLAGS ?= $(ADDFLAGS) -ldflags '-w -s $(LDFLAGS)'
RELEASE=teleport-ent-$(GITTAG)-$(ARCH)-bin

LIBSE = $(shell find lib -type f -name '*.go')
OSSLIBS = $(shell find ../lib -type f -name '*.go')

TCTLSRCE = $(shell find tool/tctl -type f -name '*.go')
TELEPORTSRCE = $(shell find tool/teleport -type f -name '*.go')

# PAM support will only be built into Teleport if headers exist at build time.
ifneq ("$(wildcard /usr/include/security/pam_appl.h)","")
PAMFLAGS = -tags pam
endif

# All builds all binaries
.PHONY: all
all:
	$(MAKE) -C ..
	$(MAKE) $(BUILDDIR)/teleport $(BUILDDIR)/tctl

$(BUILDDIR)/tctl: $(LIBSE) $(OSSLIBS) $(TCTLSRCE)
	go build $(PAMFLAGS) -o $(BUILDDIR)/tctl -i $(BUILDFLAGS) ./tool/tctl

$(BUILDDIR)/teleport: $(LIBSE) $(OSSLIBS) $(TELEPORTSRCE)
	go build $(PAMFLAGS) -o $(BUILDDIR)/teleport -i $(BUILDFLAGS) ./tool/teleport

# make clean: cleans the build
.PHONY: clean
clean:
	rm -rf $(BUILDDIR)
	rm -rf *gz

# make release - produces a binary release tarball
# (must be run as a sub-make)
.PHONY:
release: clean full
	cp ../README.md $(BUILDDIR)/
	cp ../CHANGELOG.md $(BUILDDIR)/
	cp ../build.assets/install $(BUILDDIR)/
	echo $(GITTAG) > $(BUILDDIR)/VERSION
	cp -rf $(BUILDDIR) teleport-ent
	cp -rf ../examples teleport-ent/
	cp ../build/tsh teleport-ent/
	tar -czf $(RELEASE).tar.gz teleport-ent
	rm -rf teleport-ent
	@echo "\nCREATED: $(RELEASE).tar.gz"

# make full - produces a teleport binary with the web assets built-in
.PHONY:full
full: check-build-vars all $(BUILDDIR)/webassets.zip
	cat $(BUILDDIR)/webassets.zip >> $(BUILDDIR)/teleport
	rm -fr $(BUILDDIR)/webassets.zip
	zip -q -A $(BUILDDIR)/teleport

# build/webassets.zip archive contains the web assets (UI) which gets
# appended to teleport binary
$(BUILDDIR)/webassets.zip:
	cd web/dist ; zip -qr ../../$(BUILDDIR)/webassets.zip .


.PHONY: check-build-vars
check-build-vars:
ifndef ARCH
	$(error ARCH is not set)
endif
ifndef GITTAG
	$(error GITTAG is not set)
endif

#
# test runs teleport enterprise tests
#
.PHONY: test
test:
	go test -v ./lib/... && \
	go vet ./lib/...
