RELCLI_VERSION = prod-27567fb2801b-20240916T165602
RELCLI_DOWNLOAD_URL = https://cdn.teleport.dev/relcli-$(RELCLI_VERSION)-${OS}-$(ARCH)
RELCLI = bin/relcli-$(RELCLI_VERSION)

$(RELCLI):
	mkdir -p bin
	curl -fsSL -o "$@" "$(RELCLI_DOWNLOAD_URL)"
	chmod +x "$@"
	ls -lh "$@"

