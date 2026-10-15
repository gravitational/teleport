.PHONY: help connector plugin acl smoke users members groups destroy test

# A plain `make` prints the targets. Without this, it runs the first target,
# connector, which starts `terraform apply` against the cluster.
.DEFAULT_GOAL := help

# help prints the available targets. Keep it in sync with the targets below.
help:
	@echo "Targets:"
	@echo "  connector  Create the SAML connector CONNECTOR from IDP_METADATA"
	@echo "  plugin     Install the SCIM plugin for CONNECTOR"
	@echo "  acl        Create load test Access Lists"
	@echo "  smoke      Run smoke test"
	@echo "  users      Run users load test"
	@echo "  members    Run members load test"
	@echo "  groups     Run get groups load test"
	@echo "  destroy    Delete users, Access Lists, plugin and connector (needs USER_COUNT)"
	@echo "  test       Run the fixture unit tests"

K6 ?= k6
RESULTS ?= results
# NAME names the load test run's output files: results/<NAME>.{json,log,html}. Defaults to the
# make target, plus -<OP> when OP is set (e.g. users-delete).
NAME ?= $@$(if $(OP),-$(OP))

export K6_WEB_DASHBOARD ?= true

# TELEPORT_PROXY is the Teleport Proxy address with port, e.g. example.teleport.sh:443.
ifndef TELEPORT_PROXY
TELEPORT_PROXY := $(shell tsh env 2>/dev/null | sed -n 's/^export TELEPORT_PROXY=//p')
endif
CONNECTOR ?= loadtest
IDP_METADATA ?= terraform/connector/idp-metadata.xml
# ACL owner defaults to first load test user account.
ACL_OWNER ?= k6-user-0@example.com
ACL_COUNT ?=

# SMOKE_GROUP is the Access List name in SMOKE_ACL that `make smoke` creates and deletes. 
# Keep the name in sync with assets/acl-smoke.yaml and SMOKE_GROUP_NAME in lib/fixture.js.
SMOKE_GROUP = k6-group-0-smoke
SMOKE_ACL = assets/acl-smoke.yaml

# REQUIRE_PROXY stops a target when TELEPORT_PROXY is not set.
REQUIRE_PROXY = @test -n "$(TELEPORT_PROXY)" || { echo "run tsh login, or set TELEPORT_PROXY, e.g. example.teleport.sh:443"; exit 1; }

# Each Terraform configuration has its own state, so each target changes only its
# own resources. 

TF_CONNECTOR = TF_VAR_proxy_addr=$(TELEPORT_PROXY) \
     TF_VAR_connector_name=$(CONNECTOR) \
     TF_VAR_entity_descriptor_file=$(abspath $(IDP_METADATA)) \
     terraform -chdir=terraform/connector

TF_ACCESS_LISTS = TF_VAR_proxy_addr=$(TELEPORT_PROXY) \
     TF_VAR_acl_owner=$(ACL_OWNER) \
     TF_VAR_acl_count=$(ACL_COUNT) \
     terraform -chdir=terraform/acl

K6_RUN = mkdir -p $(RESULTS) && \
      K6_WEB_DASHBOARD_EXPORT=$(RESULTS)/$(NAME).html \
      $(K6) run --summary-export=$(RESULTS)/$(NAME).json --console-output=$(RESULTS)/$(NAME).log --no-color

# connector creates the SAML connector CONNECTOR configured with IDP_METADATA using Terraform.
# Skip it if the cluster already has a connector you want to use (see `make plugin`).
connector:
	$(REQUIRE_PROXY)
	$(TF_CONNECTOR) init -input=false
	$(TF_CONNECTOR) apply

# plugin installs the SCIM plugin for an existing connector CONNECTOR, e.g.,
# `make plugin CONNECTOR=okta`.
plugin:
	tctl plugins install scim --connector=$(CONNECTOR)

# acl creates the load test Access Lists k6-group-0 .. k6-group-<ACL_COUNT-1>.
# ACL_COUNT has no default, so that a run without it cannot delete lists.
# In a consecutive run, a lower ACL_COUNT deletes the lists above it.
acl:
	$(REQUIRE_PROXY)
	@test -n "$(ACL_COUNT)" || { echo "set ACL_COUNT to the number of load-test Access Lists, e.g. 100"; exit 1; }
	$(TF_ACCESS_LISTS) init -input=false
	$(TF_ACCESS_LISTS) apply

# smoke creates the Access List SMOKE_GROUP from SMOKE_ACL with tctl, runs
# scim-smoke.js, and deletes the list again. k6 itself creates and deletes the
# smoke users.
smoke:
	tctl create -f $(SMOKE_ACL)
	status=0; \
	$(K6_RUN) scim-smoke.js || status=$$?; \
	tctl rm access_list/$(SMOKE_GROUP) || { test $$status -ne 0 || status=1; }; \
	exit $$status

users:
	$(K6_RUN) scim-users.js

members:
	$(K6_RUN) scim-members.js

groups:
	$(K6_RUN) scim-groups.js

# destroy deletes Teleport resources in this order:
#  1. Deletes users using SCIM.
#  2. Deletes Access Lists using Terraform.
#  3. Deletes SCIM plugin using tctl.
#  4. Deletes connector using Terraform.
destroy:
	$(REQUIRE_PROXY)
	@test -n "$(USER_COUNT)" || { echo "set USER_COUNT to the number of load test users to delete, e.g. 1000"; exit 1; }
	$(MAKE) users OP=delete MAX_RETRIES=$(or $(MAX_RETRIES),20)
	@if test -f terraform/acl/terraform.tfstate; then \
	        $(TF_ACCESS_LISTS) destroy; \
	else \
	        echo "no Terraform state in terraform/acl, skip Access Lists"; \
	fi
	tctl plugins delete scim
	@if test -f terraform/connector/terraform.tfstate; then \
	        $(TF_CONNECTOR) destroy; \
	else \
	        echo "no Terraform state in terraform/connector, skip connector"; \
	fi

# test runs the fixture unit tests.
test:
	node --test lib/fixture.test.js
