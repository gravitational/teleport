.PHONY: test
test: test-access test-lib test-event-handler test-terraform-provider

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

.PHONY: release-event-handler
	$(MAKE) -C event-handler release

define access_plugin_template
.PHONY: release-access-$(1)
release-access-$(1):
	$(MAKE) -C access/$(1) release
endef

ACCESS_PLUGINS = discord pagerduty msteams jira slack mattermost email datadog
$(foreach plugin,$(ACCESS_PLUGINS),$(eval $(call access_plugin_template,$(plugin))))
