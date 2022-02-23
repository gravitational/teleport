
# output:
#   grafana-pass    : admin password for grafana
#   influx-pass     : admin password for influxdb
#   influx-token    : admin token for influxdb
#   node-token      : static join token for teleport nodes
#   proxy-token     : static join token for teleport proxies
#   tc-token        : static join token for teleport trusted clusters
#   secrets.env     : env file to be used to replace placeholder values in yaml files
.PHONY:all
all: grafana-pass influx-pass influx-token join-tokens env
	@rm -rf *csr

.PHONY: env
env:
	@if [ -z ${PROXY_HOST} ]; then \
		echo "PROXY_HOST is not set, cannot apply cluster."; \
		exit 1; \
	fi

	@if [ -z ${GRAFANA_HOST} ]; then \
		echo "GRAFANA_IP is not set, cannot apply cluster."; \
		exit 1; \
	fi

	@echo PROXY_IP=$(shell make --no-print-directory -C ../../network get-proxy-ip) > secrets.env
	@echo PROXY_HOST=${PROXY_HOST} >> secrets.env
	@echo GRAFANA_IP=$(shell make --no-print-directory -C ../../network get-grafana-ip) >> secrets.env
	@echo GRAFANA_HOST=${GRAFANA_HOST} >> secrets.env
	@echo NODE_TOKEN=$(shell cat node-token) >> secrets.env
	@echo PROXY_TOKEN=$(shell cat proxy-token) >> secrets.env
	@echo TC_TOKEN=$(shell cat tc-token) >> secrets.env
	#@echo GCP_PROJECT=$(shell make --no-print-directory -C ../../cluster get-project) >> secrets.env

grafana-pass:
	openssl rand -base64 32 | tr -d '\n' > grafana-pass

influx-pass:
	openssl rand -base64 32 | tr -d '\n' > influx-pass

influx-token:
	openssl rand -base64 32 | tr -d '\n' > influx-token

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
