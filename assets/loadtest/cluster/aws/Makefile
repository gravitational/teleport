# creates an eks cluster
.PHONY: create-cluster
create-cluster:
	eksctl create cluster -f cluster.yaml

# gets kubernetes credentials
.PHONY: get-creds
get-creds:
	eksctl utils -f cluster.yaml write-kubeconfig

# deletes the eks cluster
.PHONY: destroy
destroy:
	eksctl delete cluster -f cluster.yaml --wait --disable-nodegroup-eviction
