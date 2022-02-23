# preview address configuration
.PHONY: plan
plan:
	terraform plan

# reserve static ips
.PHONY: apply
apply:
	terraform apply

# release static ips
.PHONY: destroy
destroy:
	terraform destroy

# print static ips
.PHONY: get-ips
get-ips:
	@terraform output

# print proxy static ip
.PHONY: get-proxy-ip
get-proxy-ip:
	@echo 35.82.178.61


#@terraform output -raw proxy_ip

# print grafana static ip
.PHONY: get-grafana-ip
get-grafana-ip:
	@echo 35.82.106.254


#@terraform output -raw grafana_ip
