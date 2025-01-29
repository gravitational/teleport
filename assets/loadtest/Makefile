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


# deploys nodes and adds them to the teleport cluster
.PHONY: deploy-nodes
deploy-nodes: REPLICAS ?= 100
deploy-nodes: NODES_PER_POD ?= 10
deploy-nodes:
	helm upgrade --install node-agents -n agents --create-namespace ./helm/node-agent/ --values ./helm/values/node-agents.yaml --set replicaCount=${REPLICAS} --set agentsPerPod=${NODES_PER_POD} --set proxyServer=${PROXY_SERVER} --set joinParams.token_name=${NODE_TOKEN}

# deletes all nodes
.PHONY: delete-nodes
delete-nodes:
	helm delete -n agents node-agents

# creates a local teleport user
.PHONY: create-user
create-user: TELEPORT_USER ?= test
create-user:
	kubectl --namespace teleport exec deploy/teleport-auth -- tctl users add ${TELEPORT_USER} --roles=access,editor,auditor

# creates a token that can be used by nodes to join the teleport cluster
.PHONY: create-token
create-token: TYPE ?= node,db
create-token:
	kubectl --namespace teleport exec deploy/teleport-auth -- tctl tokens add --type=${TYPE} --ttl=8h

# forwards the grafana web app to localhost:6060
.PHONY: forward-grafana
forward-grafana:
	$(MAKE) -C control-plane forward-monitor

# scales teleport auth pods up/down
.PHONY: scale-auth
scale-auth:
	kubectl scale deployment/teleport-auth --replicas=$(REPLICAS) -n teleport

.PHONY: create-rds-database
create-rds-database:
	$(MAKE) -C databases/rds apply

.PHONY: deploy-database-agents
deploy-database-agents: LOADTEST_TAG_VALUE ?= loadtest
deploy-database-agents:
ifndef TELEPORT_VERSION
	@echo "TELEPORT_VERSION is required but not provided"
	@exit 1
endif
ifndef DATABASE_ROLE_ARN
	@echo "DATABASE_ROLE_ARN is required but not provided"
	@exit 1
endif

	helm upgrade --install \
		database-agents \
		-n database-agents \
		--create-namespace \
		teleport/teleport-kube-agent \
		--set teleportVersionOverride="${TELEPORT_VERSION}" \
		--set highAvailability.replicaCount=${NODE_REPLICAS} \
		--set roles=db \
		--set proxyAddr=${PROXY_SERVER} \
		--set authToken=${NODE_TOKEN} \
		--set annotations.serviceAccount."eks\.amazonaws\.com/role-arn"=${DATABASE_ROLE_ARN} \
		--set awsDatabases[0].types[0]=rdsproxy \
		--set awsDatabases[0].regions[0]=us-east-1 \
		--set awsDatabases[0].tags."loadtest"="${LOADTEST_TAG_VALUE}"

	$(MAKE) -C control-plane enable-database-agents-monitoring

# deploys pods with self-contained teleport clusters that can later be connected to the main cluster with "make join-tc"
.PHONY: deploy-tc
deploy-tc: REPLICAS ?= 500
deploy-tc:
	helm upgrade --install trusted-cluster -n tc --create-namespace ./helm/trusted-cluster --values ./helm/values/trusted-clusters.yaml --set proxy=${PROXY_SERVER} --set token=${TOKEN} --set replicaCount=${REPLICAS} --set root=${ROOT_CLUSTER}

# joins all deployed leaf clusters to the root cluster
.PHONY: join-tc
join-tc:
	kubectl get pod -n tc -o name --no-headers \
    	| xargs -P 10 -n 1 -I {} kubectl -n tc exec {} -- tctl create -f /etc/teleport/tc.yaml

# deletes trusted clusters
.PHONY: delete-tc
delete-tc:
	helm delete -n tc trusted-cluster
	kubectl --namespace teleport exec deploy/teleport-auth \
		-- /busybox/sh -c "tctl get rc | grep ' name:' | cut -d ':' -f2- | xargs -P 20 -n 1 -I {} tctl rm rc/{}"

# creates a bot used by the soak tests to authenticate with the cluster
.PHONY: create-soaktest-bot
create-soaktest-bot:
	tctl bots add --roles=access --logins=root soaktest-bot

# removes the soak test bot
.PHONY: delete-soaktest-bot
delete-soaktest-bot:
	tctl bots rm soaktest-bot

# deploys a job that runs a series of tsh bench ssh commands
.PHONY: deploy-soaktest
deploy-soaktest:
	helm upgrade --install soaktest -n soaktest --create-namespace ./helm/soaktest --values ./helm/values/soaktest.yaml --set proxy=${PROXY_SERVER} --set token=${TOKEN} --set label=${LABEL} --set node=${NODE} --set duration=${DURATION}

# terminates the soak test job
.PHONY: delete-soaktest
delete-soaktest:
	helm delete -n soaktest soaktest
