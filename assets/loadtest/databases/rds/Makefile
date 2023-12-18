REGION := us-east-1
PREFIX := loadtest
EKS_CLUSTER_NAME := ""
TF_VARS = -var "region=$(REGION)" \
		-var "prefix=$(PREFIX)" \
		-var "eks_cluster_name=$(EKS_CLUSTER_NAME)"

.PHONY: apply
apply:
	@terraform init
	terraform apply $(TF_VARS)

.PHONY: destroy
destroy:
	terraform destroy $(TF_VARS)
