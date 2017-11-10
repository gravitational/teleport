REGION ?=
TABLE_NAME ?=
export

.PHONY: autoscale-plan
autoscale-plan:
	cd autoscale && terraform init
	cd autoscale && TF_VAR_table_name=$(TABLE_NAME) TF_VAR_region=$(REGION) terraform plan

.PHONY: autoscale-apply
autoscale-apply:
	cd autoscale && terraform init
	cd autoscale && TF_VAR_table_name=$(TABLE_NAME) TF_VAR_region=$(REGION) terraform apply


.PHONY: autoscale-destroy
autoscale-destroy:
	cd autoscale && terraform init
	cd autoscale && TF_VAR_table_name=$(TABLE_NAME) TF_VAR_region=$(REGION) terraform destroy
