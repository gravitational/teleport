LICENSE_PATH ?= /var/lib/teleport/license.pem
CERT_MANAGER_VERSION ?= v1.7.1
SOAK_TEST_DURATION ?= 30m
BACKEND ?= etcd
USE_CERT_MANAGER ?= yes
TELEPORT_IMAGE ?= quay.io/gravitational/teleport-ent:9.0.0
NAMESPACE ?= loadtest

# performs initialization needed for cluster
# 1) generates etcd certs
# 2) creates loadtest namespace
# 3) installs cert-manager
# 4) creates and applies secrets
.PHONY: setup
setup:
	@if [ -z ${TELEPORT_IMAGE} ]; then \
		echo "TELEPORT_IMAGE is not set, cannot apply cluster."; \
		exit 1; \
	fi


	@echo "applying image: ${TELEPORT_IMAGE}"
ifeq ($(BACKEND), etcd)
	make -C ../etcd/certs all
endif
	kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	make -C ./secrets all
ifeq ($(USE_CERT_MANAGER), yes)
	kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml
endif
	make generate-secrets

# create kubernetes secrets 
.PHONY: generate-secrets
generate-secrets:
ifeq ($(BACKEND), etcd)
	kubectl create secret generic etcd-client-certs -n $(NAMESPACE) \
		--from-file=client-cert.pem=../etcd/certs/client-cert.pem \
		--from-file=client-key.pem=../etcd/certs/client-key.pem \
		--from-file=ca-cert.pem=../etcd/certs/ca-cert.pem \
		--dry-run=client -o yaml | kubectl apply -f -

	kubectl create secret generic etcd-server-certs -n $(NAMESPACE) \
		--from-file=server-cert.pem=../etcd/certs/server-cert.pem \
		--from-file=server-key.pem=../etcd/certs/server-key.pem \
		--from-file=ca-cert.pem=../etcd/certs/ca-cert.pem \
		--dry-run=client -o yaml | kubectl apply -f -
endif

ifeq ($(BACKEND), firestore)
	kubectl create secret generic gcp-creds -n $(NAMESPACE) \
		--from-file=gcp_creds.json=${GCP_CREDS_LOCATION} \
		--dry-run=client -o yaml | kubectl apply -f -
endif

	kubectl create secret generic license -n $(NAMESPACE) \
		--from-file=license.pem=$(LICENSE_PATH) \
		--dry-run=client -o yaml | kubectl apply -f -

# deletes the loadtest and cert-manager namespaces
.PHONY: clean
clean:
	make -C secrets clean
	make -C ../etcd/certs clean
	kubectl delete namespace $(NAMESPACE) --ignore-not-found
	kubectl delete -f https://github.com/jetstack/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml --ignore-not-found


ifeq ($(BACKEND), etcd)
# deploys etcd and teleport to the loadtest namespace
.PHONY: apply
apply: setup install-etcd generate-certificates install-monitor install-teleport
else
# deploys and teleport to the loadtest namespace
.PHONY: apply
apply: setup generate-certificates install-monitor install-teleport
endif

ifeq ($(USE_CERT_MANAGER), yes)
# generate-certificates applies a cert-manager.io/v1/ClusterIssuer and a cert-manager.io/v1/Certificate
# that will automatically fetch tls certificates
.PHONY: generate-certificates
generate-certificates:
	kubectl wait --for=condition=available --timeout=600s deploy cert-manager -n cert-manager
	kubectl wait --for=condition=available --timeout=600s deploy cert-manager-webhook -n cert-manager
	kubectl wait --for=condition=available --timeout=600s deploy cert-manager-cainjector -n cert-manager
# we have to sleep here to due to issues with certmanager
# Error from server (InternalError): error when creating "certificate.yaml": Internal error occurred: failed calling webhook "webhook.cert-manager.io": Post "https://cert-manager-webhook.cert-manager.svc:443/mutate?timeout=10s": x509: certificate signed by unknown authority
# Error from server (InternalError): error when creating "certificate.yaml": Internal error occurred: failed calling webhook "webhook.cert-manager.io": Post "https://cert-manager-webhook.cert-manager.svc:443/mutate?timeout=10s": x509: certificate signed by unknown authority
# See https://github.com/jetstack/cert-manager/issues/2602 and https://github.com/jetstack/cert-manager/issues/2752
	@echo "waiting for cert-manager to be ready..."
	@sleep 90
	kubectl apply -f certificate.yaml
else
.PHONY: generate-certificates
generate-certificates:
endif

# installs teleport auth, proxy, one IoT node and one non-IoT node
.PHONY: install-teleport
install-teleport: install-auth install-proxy install-node install-iot-node

# deletes teleport deployments, services, and configmaps
.PHONY: delete-teleport
delete-teleport: delete-tc delete-nodes delete-proxy delete-auth

# installs prometheus exporter
.PHONY: install-monitor
install-monitor:
	@make expand-yaml FILENAME=prometheus NAMESPACE=$(NAMESPACE)
	kubectl apply -f prometheus-gen.yaml

# deletes prometheus exporter
.PHONY: delete-monitor
delete-monitor:
	kubectl delete -f prometheus-gen.yaml --ignore-not-found

# installs an etcd cluster
.PHONY: install-etcd
install-etcd:
	kubectl apply -f etcd.yaml

# deletes etcd deployment, services, and configmaps
.PHONY: delete-etcd
delete-etcd:
	kubectl delete -f etcd.yaml --ignore-not-found


# install auth and applies required teleport resources for loadtests
.PHONY: install-auth
install-auth: setup-auth
	kubectl wait --for=condition=ready pod -l teleport-role=auth -n $(NAMESPACE) --timeout=120s

	kubectl -n $(NAMESPACE) exec deploy/auth -c teleport -it \
		-- tctl --config /etc/teleport/teleport.yaml create -f /etc/teleport/admin.yaml
	kubectl -n $(NAMESPACE) exec deploy/auth -c teleport -it \
		-- tctl --config /etc/teleport/teleport.yaml create -f /etc/teleport/oidc.yaml
	kubectl -n $(NAMESPACE) exec deploy/auth -c teleport -it \
		-- tctl --config /etc/teleport/teleport.yaml create -f /etc/teleport/user.yaml


ifeq ($(BACKEND), etcd)
.PHONY: setup-auth
setup-auth:
	@make expand-yaml FILENAME=../teleport/teleport-auth-etcd

	kubectl create configmap auth-config -n $(NAMESPACE) \
		--from-file=teleport.yaml=../teleport/teleport-auth-etcd-gen.yaml \
		--from-file=oidc.yaml=../teleport/oidc.yaml \
		--from-file=admin.yaml=../teleport/admin.yaml \
		--from-file=user.yaml=../teleport/soaktest-user.yaml \
		--dry-run=client -o yaml | kubectl apply -f -

	@make expand-yaml FILENAME=auth-etcd TELEPORT_IMAGE=$(TELEPORT_IMAGE) NAMESPACE=$(NAMESPACE)
	kubectl apply -f auth-etcd-gen.yaml
else ifeq ($(BACKEND), firestore)
.PHONY: setup-auth
setup-auth:
	@make expand-yaml FILENAME=../teleport/teleport-auth-firestore

	kubectl create configmap auth-config -n $(NAMESPACE) \
		--from-file=teleport.yaml=../teleport/teleport-auth-firestore-gen.yaml \
		--from-file=oidc.yaml=../teleport/oidc.yaml \
		--from-file=admin.yaml=../teleport/admin.yaml \
		--from-file=user.yaml=../teleport/soaktest-user.yaml \
		--dry-run=client -o yaml | kubectl apply -f -

	@make expand-yaml FILENAME=auth-firestore TELEPORT_IMAGE=$(TELEPORT_IMAGE) NAMESPACE=$(NAMESPACE)
	kubectl apply -f auth-firestore-gen.yaml
else ifeq ($(BACKEND), dynamo)
.PHONY: setup-auth
setup-auth:
	@make expand-yaml DYNAMO_TABLE=${DYNAMO_TABLE} DYNAMO_REGION=${DYNAMO_REGION} FILENAME=../teleport/teleport-auth-dynamo

	kubectl create configmap auth-config -n $(NAMESPACE) \
		--from-file=teleport.yaml=../teleport/teleport-auth-dynamo-gen.yaml \
		--from-file=oidc.yaml=../teleport/oidc.yaml \
		--from-file=admin.yaml=../teleport/admin.yaml \
		--from-file=user.yaml=../teleport/soaktest-user.yaml \
		--dry-run=client -o yaml | kubectl apply -f -

	@make expand-yaml FILENAME=auth-dynamo TELEPORT_IMAGE=$(TELEPORT_IMAGE) NAMESPACE=$(NAMESPACE)
	kubectl apply -f auth-dynamo-gen.yaml
else
.PHONY: setup-auth
setup-auth:
	@echo "unknown backend $(BACKEND)"
	exit 1
endif

# deletes auth deployment, services and configmaps
.PHONY: delete-auth
delete-auth:
	kubectl delete -f auth-etcd-gen.yaml --ignore-not-found
	kubectl delete -f auth-dynamo-gen.yaml --ignore-not-found
	kubectl delete -f auth-firestore-gen.yaml --ignore-not-found
	kubectl delete configmap auth-config -n $(NAMESPACE) --ignore-not-found

# install proxy
.PHONY: install-proxy
install-proxy:
	@make expand-yaml FILENAME=../teleport/teleport-proxy
	kubectl create configmap proxy-config -n $(NAMESPACE) \
		--from-file=teleport.yaml=../teleport/teleport-proxy-gen.yaml \
		--dry-run=client -o yaml | kubectl apply -f -


	@make expand-yaml FILENAME=proxy TELEPORT_IMAGE=$(TELEPORT_IMAGE) NAMESPACE=$(NAMESPACE)
	kubectl apply -f proxy-gen.yaml

# deletes proxy deployment, services and configmaps
.PHONY: delete-proxy
delete-proxy:
	kubectl delete -f proxy-gen.yaml --ignore-not-found
	kubectl delete configmap proxy-config -n $(NAMESPACE) --ignore-not-found

# deletes all node deployment and configmaps
.PHONY: delete-nodes
delete-nodes: delete-node delete-iot-node

# deletes all non-IoT nodes
.PHONY: delete-node
delete-node:
	kubectl delete -f node-gen.yaml --ignore-not-found
	kubectl delete configmap node-config -n $(NAMESPACE) --ignore-not-found

# deletes all IoT nodes
.PHONY: delete-iot-node
delete-iot-node:
	kubectl delete -f iot-node-gen.yaml --ignore-not-found
	kubectl delete configmap iot-node-config -n $(NAMESPACE) --ignore-not-found

# install one IoT node and one non-IoT node
.PHONY: install-nodes
install-nodes: install-iot-node install-node

# install an IoT mode node
.PHONY: install-iot-node
install-iot-node:
	@make expand-yaml FILENAME=../teleport/teleport-iot-node
	kubectl create configmap iot-node-config -n $(NAMESPACE) \
		--from-file=teleport.yaml=../teleport/teleport-iot-node-gen.yaml \
		--dry-run=client -o yaml | kubectl apply -f -

	@make expand-yaml FILENAME=iot-node TELEPORT_IMAGE=$(TELEPORT_IMAGE) NAMESPACE=$(NAMESPACE)
	kubectl apply -f iot-node-gen.yaml

# install a non-IoT mode node
.PHONY: install-node
install-node:
	@make expand-yaml FILENAME=../teleport/teleport-node
	kubectl create configmap node-config -n $(NAMESPACE) \
		--from-file=teleport.yaml=../teleport/teleport-node-gen.yaml \
		--dry-run=client -o yaml | kubectl apply -f -

	@make expand-yaml FILENAME=node TELEPORT_IMAGE=$(TELEPORT_IMAGE) NAMESPACE=$(NAMESPACE)
	kubectl apply -f node-gen.yaml

# installs a trusted cluster
.PHONY: install-tc
install-tc:
	@make expand-yaml FILENAME=../teleport/tc
	kubectl create configmap tc-config -n $(NAMESPACE) \
		--from-file=teleport.yaml=../teleport/teleport-tc.yaml \
		--from-file=cluster.yaml=../teleport/tc-gen.yaml \
		--dry-run=client -o yaml | kubectl apply -f -

	@make expand-yaml FILENAME=tc TELEPORT_IMAGE=$(TELEPORT_IMAGE) NAMESPACE=$(NAMESPACE) NAMESPACE=$(NAMESPACE)
	kubectl apply -f tc-gen.yaml

# deletes all rc resources from teleport and deletes trusted cluster deployments and configmaps
.PHONY: delete-tc
delete-tc:
	kubectl delete -f tc-gen.yaml --ignore-not-found
	kubectl delete configmap tc-config -n $(NAMESPACE) --ignore-not-found

	kubectl -n $(NAMESPACE) exec deploy/auth -c teleport -it \
		-- /bin/bash -c "tctl --config /etc/teleport/teleport.yaml get rc | grep ' name:' | cut -d ':' -f2- | xargs -P 20 -n 1 -I {} tctl --config /etc/teleport/teleport.yaml rm rc/{}"

# joins all trusted clusters to root cluster
.PHONY: setup-tc
setup-tc:
	kubectl get pod -n $(NAMESPACE) -l app="tc" -o custom-columns=name:metadata.name --no-headers \
    	| xargs -P 20 -n 1 -I {} kubectl -n $(NAMESPACE) exec {} -- tctl --config /etc/teleport/teleport.yaml create -f /etc/teleport/cluster.yaml

# scales trusted clusters to 500
.PHONY: scale-tc-500
scale-tc-500:
	kubectl scale --replicas=500 deploy tc -n $(NAMESPACE)

# scales trusted clusters to 1
.PHONY: scale-tc-1
scale-tc-1:
	kubectl scale --replicas=1 deploy tc -n $(NAMESPACE)

# scales nodes to 1
.PHONY: scale-1-non-iot
scale-1-non-iot:
	kubectl scale --replicas=1 deploy node -n $(NAMESPACE)

# scales nodes to 1000
.PHONY: scale-1k-non-iot
scale-1k-non-iot:
	kubectl scale --replicas=1000 deploy node -n $(NAMESPACE)

# scales nodes to 10000
.PHONY: scale-10k-non-iot
scale-10k-non-iot:
	kubectl scale --replicas=10000 deploy node -n $(NAMESPACE)

# scales nodes to 1
.PHONY: scale-1-iot
scale-1-iot:
	kubectl scale --replicas=1 deploy iot-node -n $(NAMESPACE)

# scales nodes to 1000
.PHONY: scale-1k-iot
scale-1k-iot:
	kubectl scale --replicas=1000 deploy iot-node -n $(NAMESPACE)

# scales nodes to 10000
.PHONY: scale-10k-iot
scale-10k-iot:
	kubectl scale --replicas=10000 deploy iot-node -n $(NAMESPACE)

# gets pods in loadtest namespace
.PHONY: pods
pods:
	kubectl get pods -n $(NAMESPACE)

# removes all soak test jobs and configmaps
.PHONY: delete-soaktest
.PHONY: delete-soaktest
delete-soaktest:
	kubectl delete job -l app=soaktest -n $(NAMESPACE) --ignore-not-found

	kubectl delete configmap soaktest-config -n $(NAMESPACE) --ignore-not-found

# creates the soak test job
.PHONY: install-soaktest
install-soaktest:
	kubectl create configmap soaktest-config -n $(NAMESPACE) \
		--from-file=soaktest.sh=../teleport/soaktest.sh \
		--from-literal=DURATION=$(SOAK_TEST_DURATION) \
		--dry-run=client -o yaml | kubectl apply -f -

	kubectl create secret generic soaktest -n $(NAMESPACE) \
		--from-file=auth=./secrets/soaktest-auth \
		--from-literal=PROXY_HOST=$$(cat ./secrets/secrets.env | grep PROXY_HOST | cut -d '=' -f2- ) \
		--dry-run=client -o yaml | kubectl apply -f -

	@make expand-yaml FILENAME=soaktest TELEPORT_IMAGE=$(TELEPORT_IMAGE) NAMESPACE=$(NAMESPACE)
	kubectl create -f soaktest-gen.yaml

# deploys a job to run the soak tests
.PHONY: run-soak-tests
run-soak-tests:
	kubectl -n $(NAMESPACE) exec $$(kubectl get pod -n $(NAMESPACE) -l teleport-role="auth" -o jsonpath="{.items[0].metadata.name}") -c teleport -it \
		-- tctl auth sign --overwrite --user=soaktest-runner --out=/data/soaktest-auth --ttl=8760h --config /etc/teleport/teleport.yaml

	kubectl cp -c teleport loadtest/$$(kubectl get pod -n $(NAMESPACE) -l teleport-role="auth" -o jsonpath="{.items[0].metadata.name}"):/data/soaktest-auth ./secrets/soaktest-auth

	kubectl wait --for=condition=available --timeout=600s deploy/node -n $(NAMESPACE)
	kubectl wait --for=condition=available --timeout=600s deploy/iot-node -n $(NAMESPACE)

	@make install-soaktest

	@sleep 1

	kubectl wait --for=condition=ready pod $$(kubectl get pods --sort-by=.metadata.creationTimestamp -o jsonpath="{.items[-1:].metadata.name}" -l app=soaktest -n $(NAMESPACE)) -n $(NAMESPACE) --timeout=120s
	kubectl logs $$(kubectl get pods --sort-by=.metadata.creationTimestamp -o jsonpath="{.items[-1:].metadata.name}" -l app=soaktest -n $(NAMESPACE)) -n $(NAMESPACE) --tail -1 -f

# runs the node scaling tests
.PHONY: run-scaling-test
run-scaling-test:
	@make delete-nodes
	@make install-node
	@make scale-10k-non-iot
	@kubectl wait --for=condition=available deploy/node -n $(NAMESPACE) --timeout=60m
	@sleep 30
	@make scale-1-non-iot
	@sleep 15
	@make scale-10k-non-iot
	@kubectl wait --for=condition=available deploy/node -n $(NAMESPACE) --timeout=60m
	@sleep 15
	@make scale-1-non-iot

	@sleep 15

	@make delete-nodes
	@make install-iot-node
	@make scale-10k-iot
	@kubectl wait --for=condition=available deploy/iot-node -n $(NAMESPACE) --timeout=60m
	@sleep 30
	@make scale-1-iot
	@sleep 15
	@make scale-10k-iot
	@kubectl wait --for=condition=available  deploy/iot-node -n $(NAMESPACE) --timeout=60m
	@sleep 15
	@make scale-1

	@make delete-nodes
	@make install-nodes

# runs the trusted-cluster scaling tests
.PHONY: run-tc-scaling-test
run-tc-scaling-test:
	@make install-tc
	@make scale-tc-500
	kubectl wait --for=condition=available deploy/tc -n $(NAMESPACE) --timeout=60m
	@sleep 120
	@make setup-tc

	@sleep 1200

	@make delete-tc

	@sleep 180

	@make install-tc
	@make scale-tc-500
	kubectl wait --for=condition=available deploy/tc -n $(NAMESPACE) --timeout=60m
	@sleep 120
	@make setup-tc
	
	@sleep 1200
	
	@make delete-tc

# collect goroutine and heap go profiles from the auth deployment
.PHONY: collect-profiles
collect-profiles:
	kubectl port-forward service/auth 3434:3434 -n $(NAMESPACE) > /dev/null 2>&1 &

	@echo "waiting for auth to be available..."

	@timeout 30 sh -c 'until nc -z localhost 3434; do sleep 0.5; done'

	@make fetch-profiles LOCATION=auth-$(shell date +%s)

	kill -s kill $$(pgrep -f 3434:3434)

	kubectl port-forward service/proxy 3434:3434 -n $(NAMESPACE) > /dev/null 2>&1 &

	@echo "waiting for proxy to be available..."

	@timeout 30 sh -c 'until nc -z localhost 3434; do sleep 0.5; done'

	@make fetch-profiles LOCATION=proxy-$(shell date +%s)

	kill -s kill $$(pgrep -f 3434:3434)

# downloads the remote profiles
.PHONY: fetch-profiles
fetch-profiles:
	mkdir -p $(shell pwd)/profiles/$(LOCATION)/
	curl -o $(shell pwd)/profiles/$(LOCATION)/goroutine.profile http://127.0.0.1:3434/debug/pprof/goroutine
	curl -o $(shell pwd)/profiles/$(LOCATION)/heap.profile http://127.0.0.1:3434/debug/pprof/heap

# expands any placeholders in the provided yaml file with the value in the matching environment variable. the
# output file will be named the same with a -gen suffix, i.e input = test then output will be test-gen.yaml
.PHONY: expand-yaml
expand-yaml:
	@bash -c "set -a && source ./secrets/secrets.env && set +a && envsubst < $(FILENAME).yaml > $(FILENAME)-gen.yaml"