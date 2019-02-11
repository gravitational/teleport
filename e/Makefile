# This makefile usually runs as a sub-make by ../Makefile
# But running regular 'make' and 'make clean' works.
BUILDDIR ?= build
BUILDFLAGS ?= $(ADDFLAGS) -ldflags '-w -s $(LDFLAGS)'
CGOFLAG ?= CGO_ENABLED=1
RELEASE := teleport-ent-$(GITTAG)-$(OS)-$(ARCH)-bin
BINARIES ?= $(BUILDDIR)/tctl $(BUILDDIR)/teleport

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)

# PAM support will only be built into Teleport if headers exist at build time.
ifneq ("$(wildcard /usr/include/security/pam_appl.h)","")
PAMFLAGS = -tags pam
endif

# All build Enterprise version of "teleport" and "tctl" binaries and copy
# over tsh from OSS.
.PHONY: all
all:
	@echo "---> Building Enterprise binaries."
	$(MAKE) $(BINARIES)

# By making these 3 targets below (tsh, tctl and teleport) PHONY we are solving
# several problems:
# * Build will rely on go build internal caching https://golang.org/doc/go1.10 at all times
# * Manual change detection was broken on a large dependency tree
# If you are considering changing this behavior, please consult with dev team first
.PHONY: $(BUILDDIR)/tctl
$(BUILDDIR)/tctl:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build $(PAMFLAGS) -o $(BUILDDIR)/tctl $(BUILDFLAGS) ./tool/tctl

.PHONY: $(BUILDDIR)/teleport
$(BUILDDIR)/teleport:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build $(PAMFLAGS) -o $(BUILDDIR)/teleport $(BUILDFLAGS) ./tool/teleport

# The tsh target does not actually build an Enterprise version of tsh, it just
# copies it over from OSS.
.PHONY: $(BUILDDIR)/tsh
$(BUILDDIR)/tsh:
	cp ../build/tsh $(BUILDDIR)/

# make clean: cleans the build
.PHONY: clean
clean:
	@echo "---> Cleaning up Enterprise build artifacts."
	rm -rf $(BUILDDIR)
	rm -rf *gz

# make release - Produces a binary release tarball.
.PHONY:
export
release:
	@echo
	@echo "---> $(RELEASE_MESSAGE)"
	$(MAKE) --no-print-directory release-unix

# make release-unix - Produces a binary release tarball containing teleport,
# tctl, and tsh.
.PHONY:
release-unix: clean full
	@echo "---> Creating Enterprise release archive."
	cp ../README.md $(BUILDDIR)/
	cp ../CHANGELOG.md $(BUILDDIR)/
	cp ../build.assets/install $(BUILDDIR)/
	echo $(GITTAG) > $(BUILDDIR)/VERSION
	cp -rf $(BUILDDIR) teleport-ent
	cp -rf ../examples teleport-ent/
	cp ../build/tsh teleport-ent/
	tar -czf $(RELEASE).tar.gz teleport-ent
	rm -rf teleport-ent
	@echo "---> Created $(RELEASE).tar.gz."

# make full - produces a teleport binary with the web assets built-in
.PHONY:full
full: check-build-vars all $(BUILDDIR)/webassets.zip
	@echo "---> Attaching Enterprise web assets."
	cat $(BUILDDIR)/webassets.zip >> $(BUILDDIR)/teleport
	rm -fr $(BUILDDIR)/webassets.zip
	zip -q -A $(BUILDDIR)/teleport

# build/webassets.zip archive contains the web assets (UI) which gets
# appended to teleport binary
$(BUILDDIR)/webassets.zip:
	@echo "---> Building Enterprise web assets."
	cd web/dist ; zip -qr ../../$(BUILDDIR)/webassets.zip .

.PHONY: check-build-vars
check-build-vars:
ifndef OS
	$(error OS is not set)
endif
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

.PHONY: image
image:
	cp ../build.assets/charts/Dockerfile $(BUILDDIR)/
	cd $(BUILDDIR) && docker build --no-cache . -t quay.io/gravitational/teleport-ent:$(VERSION)

.PHONY: publish
publish:
	docker push quay.io/gravitational/teleport-ent:$(VERSION)

# Build chart
.PHONY: chart
chart: VERSION := $(shell $(MAKE) -s -C .. print-version)
chart:
	rm -f teleport-$(VERSION).tgz
	helm package ../examples/chart/teleport --version=$(VERSION)

# install-chart
.PHONY:
install-chart: VERSION := $(shell $(MAKE) -s -C .. print-version)
install-chart: chart
	helm init
	helm install teleport-$(VERSION).tgz -n teleport

# debug-chart
.PHONY:
debug-chart: VERSION := $(shell $(MAKE) -s -C .. print-version)
debug-chart:
	helm init
	helm install --dry-run --debug teleport-$(VERSION).tgz -n teleport

# uninstall-chart
.PHONY:
uninstall-chart:
	helm del --purge teleport

# upgrade chart
.PHONY:
upgrade-chart: VERSION := $(shell $(MAKE) -s -C .. print-version)
upgrade-chart: chart
	helm upgrade teleport teleport-$(VERSION).tgz
