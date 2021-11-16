SOAK_TEST_DURATION ?= 30m
USE_CERT_MANAGER ?= yes

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
	@make -C k8s apply BACKEND=etcd USE_CERT_MANAGER=$(USE_CERT_MANAGER)

# deploy teleport with etcd backend to loadtest namespace
.PHONY: deploy-firestore-cluster
deploy-firestore-cluster:
	@make -C k8s apply BACKEND=firestore USE_CERT_MANAGER=$(USE_CERT_MANAGER)

# delete the loadtest namespace
.PHONY: delete-deploy
delete-deploy:
	@make -C k8s clean USE_CERT_MANAGER=$(USE_CERT_MANAGER)

# run soak tests
.PHONY: run-soak-tests
run-soak-tests:
	@make -C k8s run-soak-tests SOAK_TEST_DURATION=$(SOAK_TEST_DURATION)	

# run 500 node trusted cluster scaling test
.PHONY: run-tc-scaling-test
run-tc-scaling-test:
	@make -C k8s run-tc-scaling-test

.PHONY: run-scaling-test
run-scaling-test:
	@make -C k8s run-scaling-test

# list pods in loadtest namespace
.PHONY: pods
pods:
	@make -C k8s pods

# collect heap and goroutine profiles
.PHONY: collect-profiles
collect-profiles:
	@make -C k8s collect-profiles PROFILE_LOCATION=$(shell pwd)