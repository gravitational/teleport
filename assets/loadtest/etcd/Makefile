# Note as of 2025-08-28 this chart is no longer supported https://github.com/bitnami/containers/issues/83267
# deploys etcd into the cluster
.PHONY: deploy
deploy:
	kubectl apply -f io2.yaml
	helm --version 12.0.18 upgrade --install etcd -n etcd --create-namespace oci://registry-1.docker.io/bitnamicharts/etcd \
		--set metrics.enabled=true \
		--set metrics.podMonitor.enabled=true \
		--set auth.rbac.create=false \
		--set auth.rbac.allowNoneAuthentication=true \
		--set auth.token.enabled=false \
		--values etcd-values.yaml

# deletes etcd from the cluster
.PHONY: delete
delete:
	helm delete -n etcd etcd
	kubectl delete ns etcd
	kubectl delete -f io2.yaml
