# location of the test enterprise license file
ENTERPRISE_LICENSE ?= ../e/fixtures/license-enterprise.pem
# location of the test pro license file
PRO_LICENSE ?= ../e/fixtures/license-pro.pem
# whether to look for enterprise or pro license
MODE ?= enterprise

# include BUILDBOX_VERSION, BUILDBOX and BUILDBOX_variant variables
include ../build.assets/images.mk

#
# Default target starts two Teleport clusters
#
.PHONY: up
up: check-license
	docker-compose up

.PHONY: reup
reup: check-license
	cd .. && make
	docker-compose up

# 'make down' stops all Teleport containers, deletes them
# and their network
#
.PHONY:down
down:
	docker-compose down

# `make enter-one` gives you shell inside auth server
# of cluster "one"
#
.PHONY:enter-one
enter-one:
	docker exec -ti one /bin/bash

# `make enter-two` gives you shell inside auth server
# of cluster "two"
#
.PHONY:enter-two
enter-two:
	docker exec -ti two-auth /bin/bash

# `make enter-sshd` gives you shell inside sshd container
.PHONY:enter-sshd
enter-sshd:
	docker exec -ti one-sshd /bin/bash

# `make enter-two-proxy` gives you shell inside proxy server
# of cluster "two"
#
.PHONY:enter-two-proxy
enter-two-proxy:
	docker exec -ti two-proxy /bin/bash

# `make enter-two-node` gives you shell inside a node
# of cluster "two"
#
.PHONY:enter-two-node
enter-two-node:
	docker exec -ti two-node /bin/bash

.PHONY: export-certs
export-certs:
	docker exec -i one /bin/bash -c "tctl auth export --type=user | sed s/cert-authority\ // > /mnt/shared/certs/teleport.pub"

.PHONY: setup-tc
setup-tc:
	docker exec -i two-auth /bin/bash -c "tctl -c /root/go/src/github.com/gravitational/teleport/docker/two-auth.yaml create -f /root/go/src/github.com/gravitational/teleport/docker/two-tc.yaml"

.PHONY: delete-tc
delete-tc:
	docker exec -i two-auth /bin/bash -c "tctl -c /root/go/src/github.com/gravitational/teleport/docker/two-auth.yaml rm tc/one"

.PHONY: check-license
check-license:
	@if [ $(MODE) = "enterprise" ] && [ -f $(ENTERPRISE_LICENSE) ]; then \
		mkdir -p ./data/one && cp $(ENTERPRISE_LICENSE) ./data/one/license.pem; \
		mkdir -p ./data/two/auth && cp $(ENTERPRISE_LICENSE) ./data/two/auth/license.pem; \
	fi
	@if [ $(MODE) = "pro" ] && [ -f $(PRO_LICENSE) ]; then \
		mkdir -p ./data/one && cp $(PRO_LICENSE) ./data/one/license.pem; \
		mkdir -p ./data/two/auth && cp $(PRO_LICENSE) ./data/two/auth/license.pem; \
	fi

.PHONY: build
build:
	docker build --build-arg BUILDBOX=$(BUILDBOX) -t teleport:latest .

.PHONY: clean
clean:
	rm -rf data
