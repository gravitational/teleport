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

# deploys teleport auth and proxy with assosciated pre-requisites (monitoring/storage/etc)
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

# In order to run a large number of `tctl create` commands in-cluster we make use of a tbot deployment.
# See helm/fixtures/tbot.yaml for the token and permissions used, adjust as needed.
.PHONY: create-tbot-resources
create-tbot-resources:
	STATIC_JWKS=$$(curl $$(kubectl get --raw /.well-known/openid-configuration | jq '.issuer + "/keys"' --raw-output)); \
	sed "s/@@STATIC_JWKS@@/$${STATIC_JWKS}/g" helm/fixtures/tbot.yaml > tbot.rendered.yaml
	kubectl --namespace teleport exec -i deploy/teleport-auth -- tctl create -f < tbot.rendered.yaml

.PHONY: deploy-app-service
deploy-app-service:
	helm upgrade --install teleport-kube-agent ../../examples/chart/teleport-kube-agent \
	--create-namespace \
	--namespace teleport \
	--set roles=app \
	--set proxyAddr=${PROXY_SERVER} \
	--set authToken=${TOKEN} \
	--set "appResources[0].labels.*=*" \
	--set image=${TELEPORT_IMAGE_REPO} \
	--set teleportVersionOverride="${TELEPORT_VERSION}" \
	--values ./helm/values/app-service-agent.yaml

.PHONY: delete-app-service
delete-app-service:
	helm delete -n teleport teleport-kube-agent

.PHONY: create-app-resources
create-app-resources: COUNT ?= 100
create-app-resources: PARALLELISM ?= 10
create-app-resources:
	helm upgrade --install resources \
	-n resources --create-namespace \
	./helm/resources \
	--set proxy=${PROXY_SERVER} \
	--set clusterName=${CLUSTER_NAME} \
	--set resources.count=${COUNT} \
	--set resources.parallelism=${PARALLELISM} \
	--set resources.template=resources/apps.yaml

.PHONY: delete-resources
delete-resources:
	helm delete -n resources resources

.PHONY: deploy-windows-desktop-service
deploy-windows-desktop-service:
	helm upgrade --install teleport-windows-desktop-agent ./helm/windows-desktop-agent \
	--create-namespace \
	--namespace teleport \
	--set proxyServer=${PROXY_SERVER} \
	--set joinParams.token_name=${TOKEN} \
	--set "windowsDesktopResources[0].labels.*=*" \
	--set image.repository=${TELEPORT_IMAGE_REPO} \
	--set image.tag="${TELEPORT_VERSION}"

.PHONY: delete-windows-desktop-service
delete-windows-desktop-service:
	helm delete -n teleport teleport-windows-desktop-agent

.PHONY: create-windows-desktop-resources
create-windows-desktop-resources: COUNT ?= 100
create-windows-desktop-resources: PARALLELISM ?= 10
create-windows-desktop-resources:
	helm upgrade --install resources \
	-n resources --create-namespace \
	./helm/resources \
	--set proxy=${PROXY_SERVER} \
	--set clusterName=${CLUSTER_NAME} \
	--set resources.count=${COUNT} \
	--set resources.parallelism=${PARALLELISM} \
	--set resources.template=resources/windows-desktops.yaml

define REAL_WINDOWS_DESKTOP_YAML
kind: dynamic_windows_desktop
version: v1
metadata:
  name: real-windows-desktop
  labels:
    env: loadtest
spec:
  addr: $(REAL_WINDOWS_DESKTOP_ADDR)
  non_ad: true
endef
export REAL_WINDOWS_DESKTOP_YAML

.PHONY: create-real-windows-desktop-resource
create-real-windows-desktop-resource:
	@echo "$$REAL_WINDOWS_DESKTOP_YAML"
	echo "$$REAL_WINDOWS_DESKTOP_YAML" | tctl create -f -

.PHONY: deploy-database-service
deploy-database-service:
	helm upgrade --install teleport-database-agent ../../examples/chart/teleport-kube-agent \
	--create-namespace \
	--namespace teleport \
	--set roles=db \
	--set proxyAddr=${PROXY_SERVER} \
	--set authToken=${TOKEN} \
	--set teleportClusterName=${CLUSTER_NAME} \
	--set "databaseResources[0].labels.*=*" \
	--set image=${TELEPORT_IMAGE_REPO} \
	--set teleportVersionOverride="${TELEPORT_VERSION}" \
	--values ./helm/values/database-agent.yaml

.PHONY: delete-database-service
delete-database-service:
	helm delete -n teleport teleport-database-agent

.PHONY: create-database-resources
create-database-resources: COUNT ?= 100
create-database-resources: PARALLELISM ?= 10
create-database-resources:
	helm upgrade --install resources \
	-n resources --create-namespace \
	./helm/resources \
	--set proxy=${PROXY_SERVER} \
	--set clusterName=${CLUSTER_NAME} \
	--set resources.count=${COUNT} \
	--set resources.parallelism=${PARALLELISM} \
	--set resources.template=resources/databases.yaml

define REAL_DATABASE_YAML
kind: db
version: v3
metadata:
  name: real-postgres
  labels:
    env: loadtest
    real: "true"
spec:
  protocol: postgres
  uri: postgres-postgresql.teleport.svc.cluster.local:5432
endef
export REAL_DATABASE_YAML

.PHONY: create-real-database-resource
create-real-database-resource:
	@echo "$$REAL_DATABASE_YAML"
	echo "$$REAL_DATABASE_YAML" | tctl create -f -

# Deploys a real PostgreSQL instance and registers it with Teleport.
# Requires tctl to be authenticated. The server cert is signed by Teleport's
# DB CA and valid for 90 days (2160h).
.PHONY: deploy-postgres
deploy-postgres:
	tctl auth sign --format=db \
		--host=postgres-postgresql.teleport.svc.cluster.local \
		--out=/tmp/postgres-server \
		--ttl=2160h
	kubectl create secret generic postgres-tls -n teleport \
		--from-file=tls.crt=/tmp/postgres-server.crt \
		--from-file=tls.key=/tmp/postgres-server.key \
		--from-file=ca.crt=/tmp/postgres-server.cas \
		--dry-run=client -o yaml | kubectl apply -f -
	helm upgrade --install postgres bitnami/postgresql \
		-n teleport --create-namespace \
		--values ./helm/values/postgres.yaml
	$(MAKE) create-real-database-resource

.PHONY: delete-postgres
delete-postgres:
	helm delete -n teleport postgres
	kubectl delete secret postgres-tls -n teleport --ignore-not-found
	tctl rm db/real-postgres
