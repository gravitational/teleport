# creates an eks cluster
.PHONY: create-aws-cluster
create-aws-cluster:
	$(MAKE) -C cluster/aws create-cluster

# creates an gks cluster
.PHONY: create-gcp-cluster
create-gcp-cluster:
	$(MAKE) -C cluster/gcp create-cluster

# deletes an eks cluster
.PHONY: delete-aws-cluster
delete-aws-cluster:
	$(MAKE) -C cluster/aws destroy

# deletes a gks cluster
.PHONY: delete-gcp-cluster
delete-gcp-cluster:
	$(MAKE) -C cluster/gcp destroy

# cleans up all deployed resources in the cluster and
# any resources created in the cloud provider
.PHONY: destroy-deploy
destroy-deploy:
	$(MAKE) -C control-plane destroy

# deploys teleport auth and proxy
.PHONY: deploy-control-plane
deploy-control-plane:
	$(MAKE) -C control-plane deploy

# deletes the teleport auth and proxy deployments
.PHONY: delete-teleport
delete-teleport:
	$(MAKE) -C control-plane delete-teleport

# deploys teleport auth and proxy to the cluster
.PHONY: deploy-teleport
deploy-teleport:
	$(MAKE) -C control-plane deploy-teleport

NODE_REPLICAS ?= 1000
PROXY_SERVER ?=
NODE_TOKEN ?=

# deploys nodes and adds them to the teleport cluster
.PHONY: deploy-nodes
deploy-nodes:
	helm upgrade --install node-agents -n agents --create-namespace ./helm/node-agent/ --values ./helm/values/node-agents.yaml --set replicaCount=${NODE_REPLICAS} --set agentsPerPod=10 --set proxyServer=${PROXY_SERVER} --set joinParams.token_name=${NODE_TOKEN}

# deletes all nodes
.PHONY: delete-nodes
delete-nodes:
	helm delete -n agents node-agents

TELEPORT_USER ?= test

# creates a local teleport user
.PHONY: create-user
create-user:
	kubectl --namespace teleport exec deploy/teleport-auth -- tctl users add ${TELEPORT_USER} --roles=access,editor,auditor

# creates a token that can be used by nodes to join the
# teleport cluster
.PHONY: create-token
create-token:
	kubectl --namespace teleport exec deploy/teleport-auth -- tctl tokens add --type=node --ttl=8h

# forwards the grafana web app to localhost:6060
.PHONY: forward-grafana
forward-grafana:
	$(MAKE) -C control-plane forward-monitor

# scales teleport auth pods up/down
.PHONY: scale-auth
scale-auth:
	kubectl scale deployment/teleport-auth --replicas=$(REPLICAS) -n teleport

