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

BUILDDIR = $(CURDIR)/build
export RELEASE_DIR = $(BUILDDIR)/artifacts
$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)

.PHONY: release-event-handler
release-event-handler: | $(RELEASE_DIR)
	$(MAKE) -C event-handler release

define access_plugin_template
.PHONY: release-access-$(1)
release-access-$(1): | $(RELEASE_DIR)
	$(MAKE) -C access/$(1) release
endef

ACCESS_PLUGINS = discord pagerduty msteams jira slack mattermost email datadog
ACCESS_PLUGIN_TARGETS = $(addprefix release-access-,$(ACCESS_PLUGINS)) 
.PHONY: $(ACCESS_PLUGIN_TARGETS)
$(ACCESS_PLUGIN_TARGETS):

$(foreach plugin,$(ACCESS_PLUGINS),$(eval $(call access_plugin_template,$(plugin))))

.PHONY: release-terraform-provider
release-terraform-provider: | $(RELEASE_DIR)
	$(MAKE) -C terraform release

.PHONY: release-plugins
release-plugins: release-event-handler release-terraform-provider $(ACCESS_PLUGIN_TARGETS) 
	@echo $(ACCESS_PLUGIN_TARGETS)
