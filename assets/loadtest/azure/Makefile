.PHONY: nothing
nothing:

psql_fqdn = $(shell terraform output -raw psql_fqdn)
psql_adminuser = $(shell terraform output -raw psql_adminuser)

.PHONY: psql
psql:
	PGPASSWORD=$$(az account get-access-token --resource-type oss-rdbms --query accessToken -o tsv) psql "host=$(psql_fqdn) user=$(psql_adminuser) sslmode=require dbname=postgres"

rg_name = $(shell terraform output -raw rg_name)
aks_name = $(shell terraform output -raw aks_name)

.PHONY: aks
aks:
	az aks get-credentials --resource-group "$(rg_name)" --name "$(aks_name)"

monitoring_ns = $(shell terraform output -raw monitoring_ns)
monitoring_release = $(shell terraform output -raw monitoring_release)

.PHONY: grafana
grafana: aks
	@echo Grafana will be accessible at http://127.0.0.1:8080/
	kubectl --context "$(aks_name)" --namespace "$(monitoring_ns)" port-forward "svc/$(monitoring_release)-grafana" 8080:http-web

.PHONY: public-addr
public-addr:
	@echo $(shell terraform output -raw public_addr)

teleport_ns = $(shell terraform output -raw teleport_ns)
teleport_release = $(shell terraform output -raw teleport_release)

.PHONY: create-joe
create-joe: aks
	kubectl --context "$(aks_name)" --namespace "$(teleport_ns)" exec "deploy/$(teleport_release)-auth" -- tctl users add --roles=access,auditor,editor --logins root --kubernetes-groups system:masters joe
