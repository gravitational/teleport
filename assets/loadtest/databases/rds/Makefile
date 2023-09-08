REGION := us-east-1
PREFIX := loadtest
LOCAL_IP := $(shell curl -s https://checkip.amazonaws.com/)
EKS_CLUSTER_NAME := ""
TF_VARS = -var "region=$(REGION)" \
		-var "prefix=$(PREFIX)" \
		-var "local_ip=$(LOCAL_IP)" \
		-var "eks_cluster_name=$(EKS_CLUSTER_NAME)"

.PHONY: apply
apply:
	@terraform init
	terraform apply $(TF_VARS)

.PHONY: destroy
destroy:
	terraform destroy $(TF_VARS)
