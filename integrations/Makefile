# Set up a system-agnostic in-place sed command
IS_GNU_SED = $(shell sed --version 1>/dev/null 2>&1 && echo true || echo false)

ifeq ($(IS_GNU_SED),true)
	SED = sed -i
else
	SED = sed -i ''
endif

TELEPORT_GET_VERSION ?= v12.1.0
.PHONY: update-test-version
update-test-version:
	curl https://get.gravitational.com/teleport-{ent-,}${TELEPORT_GET_VERSION}-{darwin-amd64,linux-{amd64,arm64,arm}}-bin.tar.gz.sha256 > \
	lib/testing/integration/download_sha.dsv
	(cd ../ && \
		$(SED) 's/TELEPORT_GET_VERSION: .*/TELEPORT_GET_VERSION: $(TELEPORT_GET_VERSION)/g' .github/workflows/unit-tests-integrations.yaml \
	)

.PHONY: test
test: test-access test-lib test-event-handler

.PHONY: test-access
test-access:
	go test -v ./access/...

.PHONY: test-lib
test-lib:
	go test ./lib/...

.PHONY: test-event-handler
test-event-handler:
	make -C event-handler test
