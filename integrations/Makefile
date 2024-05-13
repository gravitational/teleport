.PHONY: test
test: test-access test-lib test-event-handler test-terraform-provider

.PHONY: test-ent
test-ent: test-access-ent test-terraform-provider-ent

.PHONY: test-full
test-full: test-access-full test-lib test-event-handler test-terraform-provider-full

.PHONY: test-access
test-access:
	go test -v ./access/...

.PHONY: test-access-ent
test-access-ent:
	go test -v ./access/... --tags enterprisetests -run 'Test.*Enterprise.*'

.PHONY: test-access-full
test-access-full:
	go test -v ./access/... --tags enterprisetests

.PHONY: test-lib
test-lib:
	go test ./lib/...

.PHONY: test-event-handler
test-event-handler:
	make -C event-handler test

.PHONY: test-terraform-provider
test-terraform-provider:
	make -C terraform test

.PHONY: test-terraform-provider-ent
test-terraform-provider-ent:
	make -C terraform test-ent

.PHONY: test-terraform-provider-full
test-terraform-provider-full:
	make -C terraform test-full
