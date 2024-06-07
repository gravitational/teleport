.PHONY: nothing
nothing:

AZ = az
KUBECTL = kubectl
PSQL = psql
TF = tofu

aks_name = $(shell $(TF) output -raw aks_name)
monitoring_namespace = $(shell $(TF) output -raw monitoring_namespace)
monitoring_release = $(shell $(TF) output -raw monitoring_release)
psql_adminuser = $(shell $(TF) output -raw psql_adminuser)
psql_fqdn = $(shell $(TF) output -raw psql_fqdn)
public_addr = $(shell $(TF) output -raw public_addr)
resource_group = $(shell $(TF) output -raw resource_group)
teleport_namespace = $(shell $(TF) output -raw teleport_namespace)
teleport_release = $(shell $(TF) output -raw teleport_release)

.PHONY: psql
psql:
	PGPASSWORD=$$($(AZ) account get-access-token --resource-type oss-rdbms --query accessToken -o tsv) $(PSQL) "host=$(psql_fqdn) user=$(psql_adminuser) sslmode=require dbname=postgres"

# this will set the current-context in kubectl, unfortunately
.PHONY: aks
aks:
	$(AZ) aks get-credentials --resource-group "$(resource_group)" --name "$(aks_name)"

.PHONY: grafana
grafana: aks
	@echo Grafana will be accessible at http://127.0.0.1:8080/
	$(KUBECTL) --context "$(aks_name)" --namespace "$(monitoring_namespace)" port-forward "svc/$(monitoring_release)-grafana" 8080:http-web

.PHONY: public-addr
public-addr:
	@echo "$(public_addr)"

.PHONY: create-joe
create-joe: aks
	$(KUBECTL) --context "$(aks_name)" --namespace "$(teleport_namespace)" exec "deploy/$(teleport_release)-auth" -- tctl users add --roles=access,auditor,editor --logins=root --kubernetes-groups=system:masters joe

.PHONY: create-token
create-token: aks
	$(KUBECTL) --context "$(aks_name)" --namespace "$(teleport_namespace)" exec "deploy/$(teleport_release)-auth" -- tctl token add --type=node --ttl=8766h

# Recursively destroying the kube cluster and everything in it is incredibly
# slow, recursively destroying the database will not work because the teleport
# user owns its databases, and tf doesn't know about the databases. Instead, we
# delete the resource group externally, then we tell tf to forget about anything
# that was in the kube cluster, and then destroy the few things remaining (DNS
# entries, optionally ACR role assignments).
.PHONY: destroy
destroy:
	if $(AZ) group show --resource-group "$(resource_group)" -o none; then $(AZ) group delete --resource-group "$(resource_group)"; fi
	$(TF) state list | grep -e ^kubernetes_ -e ^helm_ -e ^kubectl_ | xargs $(TF) state rm
	$(TF) destroy
