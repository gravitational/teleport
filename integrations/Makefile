.PHONY: test
test: test-access test-lib test-event-handler test-terraform-provider test-terraform-provider-mwi

.PHONY: test-access
test-access:
	go test -v ./access/...

.PHONY: test-lib
test-lib:
	go test ./lib/...

.PHONY: test-event-handler
test-event-handler:
	make -C event-handler test

.PHONY: test-terraform-provider
test-terraform-provider:
	make -C terraform test

.PHONY: test-terraform-provider-mwi
test-terraform-provider-mwi:
	make -C terraform-mwi test
