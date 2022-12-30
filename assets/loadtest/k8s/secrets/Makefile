
# output:
#   node-token      : static join token for teleport nodes
#   proxy-token     : static join token for teleport proxies
#   tc-token        : static join token for teleport trusted clusters
#   secrets.env     : env file to be used to replace placeholder values in yaml files
.PHONY:all
all: join-tokens env
	@rm -rf *csr

.PHONY: env
env:
	@if [ -z ${PROXY_HOST} ]; then \
		echo "PROXY_HOST is not set, cannot apply cluster."; \
		exit 1; \
	fi

	@if [ -z ${PROM_REMOTE_URL} ]; then \
		echo "PROM_REMOTE_URL is not set, cannot apply cluster."; \
		exit 1; \
	fi

	@if [ -z ${PROM_USER} ]; then \
		echo "PROM_USER is not set, cannot apply cluster."; \
		exit 1; \
	fi

	@if [ -z ${PROM_PASSWORD} ]; then \
		echo "PROM_PASSWORD is not set, cannot apply cluster."; \
		exit 1; \
	fi

	@echo PROXY_IP=$(shell make -C ../../network get-proxy-ip) > secrets.env
	@echo PROXY_HOST=${PROXY_HOST} >> secrets.env
	@echo NODE_TOKEN=$(shell cat node-token) >> secrets.env
	@echo PROXY_TOKEN=$(shell cat proxy-token) >> secrets.env
	@echo TC_TOKEN=$(shell cat tc-token) >> secrets.env
	@echo GCP_PROJECT=$(shell make -C ../../cluster get-project) >> secrets.env
	@echo PROM_REMOTE_URL=${PROM_REMOTE_URL} >> secrets.env
	@echo PROM_USER=${PROM_USER} >> secrets.env
	@echo PROM_PASSWORD=${PROM_PASSWORD} >> secrets.env

node-token:
	openssl rand -base64 32 | tr -d '\n' > node-token

proxy-token:
	openssl rand -base64 32 | tr -d '\n' > proxy-token

tc-token:
	openssl rand -base64 32 | tr -d '\n' > tc-token

.PHONY: join-tokens
join-tokens: node-token proxy-token tc-token

# removes everything
.PHONY:clean
clean:
	rm -rf *-pass *-token *-auth *.env