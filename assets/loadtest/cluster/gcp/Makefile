# preview cluster configuration
.PHONY: plan
plan:
	terraform plan

# apply cluster configuration
.PHONY: create-cluster
create-cluster:
	terraform apply

# authorize kubectl to manage the cluster
.PHONY: get-creds
get-creds:
	gcloud container clusters get-credentials $(shell make get-cluster-name) --zone us-central1 --project teleport-loadtest

# destroy the cluster
.PHONY: destroy
destroy:
	terraform destroy

# print cluster name
.PHONY: get-cluster-name
get-cluster-name:
	@terraform output -raw cluster_name

# print project name
.PHONY: get-project
get-project:
	@terraform output -raw project