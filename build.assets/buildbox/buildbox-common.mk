# Common definitions shared between makefiles

# Default parallelism of builds
NPROC ?= $(shell nproc)

# THIRDPARTY_DIR is the root of where third-party libraries and programs are
# downloaded, built and installed.
THIRDPARTY_DIR ?= /opt/thirdparty

# THIRDPARTY_PREFIX is the root of where architecture-specific third-party
# libraries are installed. Each architecture has its own directory as libraries
# are architecture-specific.
THIRDPARTY_PREFIX = $(THIRDPARTY_DIR)/$(ARCH)

# THIRDPARTY_DLDIR holds downloaded tarballs of third-party libraries and
# programs so we don't have to keep downloading them. It is safe to delete.
THIRDPARTY_DLDIR = $(THIRDPARTY_DIR)/download

# THIRDPARTY_SRCDIR is the directory where the source for third-party is
# extracted and built. Each architecture has its own extracted source as the
# build is done within the source tree.
THIRDPARTY_SRCDIR = $(THIRDPARTY_PREFIX)/src

# THIRDPARTY_HOST_PREFIX is the root of where host-specific third-party
# programs are installed, such as ct-ng and the compilers it builds. These
# run on the host that is running the build, regardless of that host
# architecture. THIRDPARTY_HOST_SRCDIR is the directory where the source
# for host-specific third-party applications is extracted and built.
THIRDPARTY_HOST_PREFIX = $(THIRDPARTY_DIR)/host
THIRDPARTY_HOST_SRCDIR = $(THIRDPARTY_HOST_PREFIX)/src

# -----------------------------------------------------------------------------
# tp-src-dir and tp-src-host-dir expand to the source directory for a third-
# party source directory which has the version of the source appended. It
# is used with `$(call ...)`, like `$(call tp-src-dir,zlib)` or
# `$(call tp-src-host-dir,crosstoolng)`.
tp-src-dir = $(THIRDPARTY_SRCDIR)/$1-$($1_VERSION)
tp-src-host-dir = $(THIRDPARTY_HOST_SRCDIR)/$1-$($1_VERSION)

# -----------------------------------------------------------------------------
# Helpers

# Create top-level directories when required
$(THIRDPARTY_SRCDIR) $(THIRDPARTY_HOST_SRCDIR) $(THIRDPARTY_DLDIR):
	mkdir -p $@

# vars for fetch-git-%. `$*` represents the `%` match.
tp-git-ref = $($*_GIT_REF)
tp-git-repo = $($*_GIT_REPO)
tp-git-ref-hash = $($*_GIT_REF_HASH)
tp-git-dl-dir = $(THIRDPARTY_DLDIR)/$*-$($*_VERSION)
tp-git-src-dir = $($*_SRCDIR)
define tp-git-fetch-cmd
	git -C "$(dir $(tp-git-dl-dir))" \
		-c advice.detachedHead=false clone --depth=1 \
		--branch=$(tp-git-ref) $(tp-git-repo) "$(tp-git-dl-dir)"
endef

# Fetch source via git.
fetch-git-%:
	mkdir -p $(dir $(tp-git-src-dir)) $(tp-git-dl-dir)
	$(if $(wildcard $(tp-git-dl-dir)),,$(tp-git-fetch-cmd))
	@if [ "$$(git -C "$(tp-git-dl-dir)" rev-parse HEAD)" != "$(tp-git-ref-hash)" ]; then \
		echo "Found unexpected HEAD commit for $(1)"; \
		echo "Expected: $(tp-git-ref-hash)"; \
		echo "Got: $$(git -C "$(tp-git-dl-dir)" rev-parse HEAD)"; \
		exit 1; \
	fi
	git clone $(tp-git-dl-dir) "$(tp-git-src-dir)"

# vars for fetch-https-%. `$*` represents the `%` match.
tp-download-url = $($*_DOWNLOAD_URL)
tp-sha1 = $($*_SHA1)
tp-download-filename = $(THIRDPARTY_DLDIR)/$(notdir $(tp-download-url))
tp-strip-components = $($*_STRIP_COMPONENTS)
tp-https-download-cmd = curl -fsSL --output "$(tp-download-filename)" "$(tp-download-url)"
tp-https-src-dir = $(call tp-src-dir,$*)
define tp-https-extract-tar-cmd
	@echo "$(tp-sha1)  $(tp-download-filename)" | sha1sum --check
	mkdir -p "$(tp-https-src-dir)"
	tar -x -a \
		--file="$(tp-download-filename)" \
		--directory="$(tp-https-src-dir)" \
		--strip-components="$(tp-strip-components)"
endef

# Fetch source tarball via https
fetch-https-%:
	@mkdir -p $(THIRDPARTY_DLDIR) $(dir $(tp-https-src-dir))
	$(if $(wildcard $(tp-download-filename)),,$(tp-https-download-cmd))
	$(if $(wildcard $(tp-https-src-dir)),,$(tp-https-extract-tar-cmd))
