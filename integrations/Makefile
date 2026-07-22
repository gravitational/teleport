TEST_LOG_DIR ?= ${abspath ./test-logs}

.PHONY: test
test: test-access test-lib test-event-handler test-terraform-provider test-terraform-provider-mwi

$(TEST_LOG_DIR):
	mkdir $(TEST_LOG_DIR)

.PHONY: test-access
test-access: | $(TEST_LOG_DIR)
	gotestsum --junitfile $(TEST_LOG_DIR)/unit-tests-integrations-access.xml --jsonfile $(TEST_LOG_DIR)/unit-tests-integrations-access.json ./access/...

.PHONY: test-lib
test-lib: | $(TEST_LOG_DIR)
	gotestsum --junitfile $(TEST_LOG_DIR)/unit-tests-integrations-lib.xml --jsonfile $(TEST_LOG_DIR)/unit-tests-integrations-lib.json ./lib/...
.PHONY: test-event-handler
test-event-handler:  | $(TEST_LOG_DIR)
	make -C event-handler test TEST_LOG_DIR=$(TEST_LOG_DIR)

.PHONY: test-terraform-provider
test-terraform-provider:  | $(TEST_LOG_DIR)
	make -C terraform test TEST_LOG_DIR=$(TEST_LOG_DIR)

.PHONY: test-terraform-provider-mwi
test-terraform-provider-mwi:  | $(TEST_LOG_DIR)
	make -C terraform-mwi test TEST_LOG_DIR=$(TEST_LOG_DIR)
