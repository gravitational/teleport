.PHONY: nothing
nothing:

psql_fqdn = $(shell terraform output -raw psql_fqdn)
psql_adminuser = $(shell terraform output -raw psql_adminuser)
resource_group = $(shell terraform output -raw resource_group)
aks_name = $(shell terraform output -raw aks_name)
monitoring_namespace = $(shell terraform output -raw monitoring_namespace)
monitoring_release = $(shell terraform output -raw monitoring_release)
teleport_namespace = $(shell terraform output -raw teleport_namespace)
teleport_release = $(shell terraform output -raw teleport_release)

.PHONY: psql
psql:
	PGPASSWORD=$$(az account get-access-token --resource-type oss-rdbms --query accessToken -o tsv) psql "host=$(psql_fqdn) user=$(psql_adminuser) sslmode=require dbname=postgres"

# this will set the current-context in kubectl, unfortunately
.PHONY: aks
aks:
	az aks get-credentials --resource-group "$(resource_group)" --name "$(aks_name)"

.PHONY: grafana
grafana: aks
	@echo Grafana will be accessible at http://127.0.0.1:8080/
	kubectl --context "$(aks_name)" --namespace "$(monitoring_namespace)" port-forward "svc/$(monitoring_release)-grafana" 8080:http-web

.PHONY: public-addr
public-addr:
	@echo $(shell terraform output -raw public_addr)

.PHONY: create-joe
create-joe: aks
	kubectl --context "$(aks_name)" --namespace "$(teleport_namespace)" exec "deploy/$(teleport_release)-auth" -- tctl users add --roles=access,auditor,editor --logins=root --kubernetes-groups=system:masters joe

.PHONY: create-token
create-token: aks
	kubectl --context "$(aks_name)" --namespace "$(teleport_namespace)" exec "deploy/$(teleport_release)-auth" -- tctl token add --type=node --ttl=8766h

# Recursively destroying the kube cluster and everything in it is incredibly
# slow, recursively destroying the database will not work because the teleport
# user owns its databases, and terraform doesn't know about the databases.
# Instead, we delete the resource group externally, then we tell terraform to
# forget about anything that was in the kube cluster, and then destroy the few
# things remaining (DNS entries, optionally ACR role assignments).
.PHONY: destroy
destroy:
	if az group show --resource-group "$(resource_group)" -o none; then az group delete --resource-group "$(resource_group)"; fi
	terraform state list | grep -e ^kubernetes_ -e ^helm_ -e ^kubectl_ | xargs terraform state rm
	terraform destroy
