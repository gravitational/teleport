SOAK_TEST_DURATION ?= 30m
USE_CERT_MANAGER ?= yes
TELEPORT_IMAGE ?= quay.io/gravitational/teleport-ent:9.0.0
NAMESPACE ?= loadtest

.PHONY: reserve-ips
reserve-ips:
	@make -C network apply

# create gke cluster 
.PHONY: create-cluster
create-cluster:
	@make -C ./cluster apply
	@make -C ./cluster get-creds

# delete gke cluster
.PHONY: delete-cluster
delete-cluster:
	@make -C cluster destroy USE_CERT_MANAGER=$(USE_CERT_MANAGER)

# deploy teleport with etcd backend to loadtest namespace
.PHONY: deploy-etcd-cluster
deploy-etcd-cluster:
	@make -C k8s apply BACKEND=etcd USE_CERT_MANAGER=$(USE_CERT_MANAGER) TELEPORT_IMAGE=$(TELEPORT_IMAGE) NAMESPACE=$(NAMESPACE)

# deploy teleport with firestore backend to loadtest namespace
.PHONY: deploy-firestore-cluster
deploy-firestore-cluster:
	@make -C k8s apply BACKEND=firestore USE_CERT_MANAGER=$(USE_CERT_MANAGER) TELEPORT_IMAGE=$(TELEPORT_IMAGE) NAMESPACE=$(NAMESPACE)

# deploy teleport with dynamo backend to loadtest namespace
.PHONY: deploy-dynamo-cluster
deploy-dynamo-cluster:
	@make -C k8s apply BACKEND=dynamo USE_CERT_MANAGER=$(USE_CERT_MANAGER) TELEPORT_IMAGE=$(TELEPORT_IMAGE) NAMESPACE=$(NAMESPACE)

# delete the loadtest namespace
.PHONY: delete-deploy
delete-deploy:
	@make -C k8s clean USE_CERT_MANAGER=$(USE_CERT_MANAGER)

# run soak tests
.PHONY: run-soak-tests
run-soak-tests:
	@make -C k8s run-soak-tests SOAK_TEST_DURATION=$(SOAK_TEST_DURATION) TELEPORT_IMAGE=$(TELEPORT_IMAGE) NAMESPACE=$(NAMESPACE)

# run 500 node trusted cluster scaling test
# This installs 500 trusted clusters, waits for a period of time and then
# deletes all 500. When they are all removed from auth the process is repeated once more.
# The test helps identify any potential memory leaks caused by remote clusters
.PHONY: run-tc-scaling-test
run-tc-scaling-test:
	@make -C k8s run-tc-scaling-test TELEPORT_IMAGE=$(TELEPORT_IMAGE) NAMESPACE=$(NAMESPACE)

# run the 10k scaling test
# This tests installs 10,000 IoT nodes and then removes all 10,000 nodes after a fixed period of time.
# The process repeats once more and then the same is done for non-IoT nodes. The test helps identify any
# potential memory leaks caused by large clusters for both non-IoT and IoT nodes.
.PHONY: run-scaling-test
run-scaling-test:
	@make -C k8s run-scaling-test TELEPORT_IMAGE=$(TELEPORT_IMAGE) NAMESPACE=$(NAMESPACE)

# list pods in loadtest namespace
.PHONY: pods
pods:
	@make -C k8s pods NAMESPACE=$(NAMESPACE)

# get cluster credentials
.PHONY: get-creds
get-creds:
	@make -C cluster get-creds

# collect heap and goroutine profiles
.PHONY: collect-profiles
collect-profiles:
	@make -C k8s collect-profiles PROFILE_LOCATION=$(shell pwd) NAMESPACE=$(NAMESPACE)