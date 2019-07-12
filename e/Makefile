# This makefile usually runs as a sub-make by ../Makefile
# But running regular 'make' and 'make clean' works.
BUILDDIR ?= build
BUILDFLAGS ?= $(ADDFLAGS) -ldflags '-w -s $(LDFLAGS)'
CGOFLAG ?= CGO_ENABLED=1
RELEASE := teleport-ent-$(GITTAG)-$(OS)-$(ARCH)-bin
BINARIES ?= $(BUILDDIR)/tctl $(BUILDDIR)/teleport
HOUSTON_DIR := "vendor/github.com/gravitational/houston/"

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)

# FIPS support must be requested at build time.
ifneq ("$(FIPS)","")
FIPS_TAG := fips
endif

# PAM support will only be built into Teleport if headers exist at build time.
ifneq ("$(wildcard /usr/include/security/pam_appl.h)","")
PAM_TAG = pam
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
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -tags "$(PAM_TAG) $(FIPS_TAG)" -o $(BUILDDIR)/tctl $(BUILDFLAGS) ./tool/tctl

.PHONY: $(BUILDDIR)/teleport
$(BUILDDIR)/teleport:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -tags "$(PAM_TAG) $(FIPS_TAG)" -o $(BUILDDIR)/teleport $(BUILDFLAGS) ./tool/teleport

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
	-rm build_houston
	-rm start_houston

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

.PHONY: integration
integration: start-houston
	go test -v ./integration/... -check.v
	$(MAKE) stop-houston

build-houston:
	sudo $(MAKE) -C $(HOUSTON_DIR) image
	@touch build-houston

start-houston: build-houston
	@docker run -d --rm=true \
		-e HOUSTON_API_HOST="localhost" \
		-e HOUSTON_WEB_HOST="web.localhost" \
		-e HOUSTON_STRIPE_SECRET_KEY="$(HOUSTON_STRIPE_SECRET_KEY)" \
		-e HOUSTON_STRIPE_PUBLIC_KEY="$(HOUSTON_STRIPE_PUBLIC_KEY)" \
		-e HOUSTON_AUTH0_CLIENT_ID="$(HOUSTON_AUTH0_CLIENT_ID)" \
		-e HOUSTON_AUTH0_CLIENT_SECRET="$(HOUSTON_AUTH0_CLIENT_SECRET)" \
		-e HOUSTON_AUTH0_AUDIENCE="$(HOUSTON_AUTH0_AUDIENCE)" \
		-e HOUSTON_AUTH0_ISSUER="$(HOUSTON_AUTH0_ISSUER)" \
		-e HOUSTON_AUTH0_AUTH_API_URL="$(HOUSTON_AUTH0_AUTH_API_URL)" \
		-e HOUSTON_AUTH0_MGMT_API_URL="$(HOUSTON_AUTH0_MGMT_API_URL)" \
		-e HOUSTON_AUTH0_REDIRECT_URL="$(HOUSTON_AUTH0_REDIRECT_URL)" \
		-e HOUSTON_ZENDESK_API_USER="$(HOUSTON_ZENDESK_API_USER)" \
		-e HOUSTON_ZENDESK_API_TOKEN="$(HOUSTON_ZENDESK_API_TOKEN)" \
		-e HOUSTON_ZENDESK_API_URL="$(HOUSTON_ZENDESK_API_URL)" \
		-e HOUSTON_GITHUB_USERNAME="$(HOUSTON_GITHUB_USERNAME)" \
		-e HOUSTON_GITHUB_ACCESS_TOKEN="$(HOUSTON_GITHUB_ACCESS_TOKEN)" \
		-e HOUSTON_SALESFORCE_CLIENT_ID="$(HOUSTON_SALESFORCE_CLIENT_ID)" \
		-e HOUSTON_SALESFORCE_CLIENT_SECRET="$(HOUSTON_SALESFORCE_CLIENT_SECRET)" \
		-e HOUSTON_SALESFORCE_USERNAME="$(HOUSTON_SALESFORCE_USERNAME)" \
		-e HOUSTON_SALESFORCE_PASSWORD="$(HOUSTON_SALESFORCE_PASSWORD)" \
		-p 10000:10000 \
	       	houston:0.0.1 /opt/houston/houston
	@touch start-houston

.PHONY: stop-houston
stop-houston:
	docker stop $$(docker ps -q --filter ancestor=houston:0.0.1)
	@rm start-houston

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

RUNTIME_SECTION ?=
TARBALL_PATH_SECTION ?=

ifneq ("$(RUNTIME)", "")
	RUNTIME_SECTION := -r $(RUNTIME)
endif
ifneq ("$(ENT_TARBALL_PATH)", "")
	TARBALL_PATH_SECTION := -s $(ENT_TARBALL_PATH)
endif

# build .pkg
.PHONY: pkg
pkg:
	cp ../build.assets/build-package.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	# arch and runtime are currently ignored on OS X
	# we pass them through for consistency - they will be dropped by the build script
	cd $(BUILDDIR) && ./build-package.sh -t ent -v $(VERSION) -p pkg -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)

# build .rpm
.PHONY: rpm
rpm:
	cp ../build.assets/build-package.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	cd $(BUILDDIR) && ./build-package.sh -t ent -v $(VERSION) -p rpm -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)

# build .deb
.PHONY: deb
deb:
	cp ../build.assets/build-package.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	cd $(BUILDDIR) && ./build-package.sh -t ent -v $(VERSION) -p deb -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)

