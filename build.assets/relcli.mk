# Makefile for downloading and using the relcli
#
# Include this file from another Makefile to utilize relcli
#	include relcli.mk
#	release: build $(RELCLI)
#		$(RELCLI) generate-manifest ...

RELCLI_VERSION = prod-27567fb2801b-20240916T165602
RELCLI_OS = $(OS:darwin=macos)
RELCLI_DOWNLOAD_URL = https://cdn.teleport.dev/relcli-$(RELCLI_VERSION)-$(RELCLI_OS)-$(ARCH)

# Defaults to build.assets/bin
RELCLI_OUTPUT_DIRECTORY = $(realpath $(dir $(lastword $(MAKEFILE_LIST))))/bin
RELCLI := $(RELCLI_OUTPUT_DIRECTORY)/relcli-$(RELCLI_VERSION)

$(RELCLI_OUTPUT_DIRECTORY):
	mkdir -p $(RELCLI_OUTPUT_DIRECTORY)

$(RELCLI): | $(RELCLI_OUTPUT_DIRECTORY)
	@echo ---> Downloading $(RELCLI)
	curl -fsSL -o "$@" "$(RELCLI_DOWNLOAD_URL)"
	chmod +x "$@"
