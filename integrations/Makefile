.PHONY: test
test: test-access test-lib test-event-handler test-terraform-provider test-terraform-provider-mwi

.PHONY: test-access
test-access:
	gotestsum --junitfile unit-tests-integrations-access.xml --jsonfile unit-tests-integrations-access.json ./access/...

.PHONY: test-lib
test-lib:
	gotestsum --junitfile unit-tests-integrations-lib.xml --jsonfile unit-tests-integrations-lib.json ./lib/...

.PHONY: test-event-handler
test-event-handler:
	make -C event-handler test

.PHONY: test-terraform-provider
test-terraform-provider:
	make -C terraform test

.PHONY: test-terraform-provider-mwi
test-terraform-provider-mwi:
	make -C terraform-mwi test
