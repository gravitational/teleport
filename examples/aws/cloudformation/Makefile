# Default region
AWS_REGION ?= us-west-2

# Cloudformation stack name to create, e.g. test1
STACK ?=

# Stack parameters, e.g ParameterKey=KeyName,ParameterValue=KeyName ParameterKey=DomainName,ParameterValue=teleport.example.com ParameterKey=DomainAdminEmail,ParameterValue=admin@example.com ParameterKey=HostedZoneID,ParameterValue=AWSZONEID
STACK_PARAMS ?=

# YAML filename
CF_YAML_FILENAME ?= ./oss.yaml
export

.PHONY: validate-template
validate-template:
	aws cloudformation validate-template --template-body file://$(CF_YAML_FILENAME)


# Stack functionality
# Create
.PHONY: create-stack
create-stack:
	$(MAKE) validate-template
	aws --region=$(AWS_REGION) cloudformation create-stack --capabilities CAPABILITY_IAM --stack-name $(STACK) --template-body file://$(CF_YAML_FILENAME) --parameters $(STACK_PARAMS)

.PHONY: create-stack-vpc
create-stack-vpc: CF_YAML_FILENAME=./vpc.yaml
create-stack-vpc:
	$(MAKE) create-stack


# Update
.PHONY: update-stack
update-stack:
	$(MAKE) validate-template
	aws --region=$(AWS_REGION) cloudformation update-stack --capabilities CAPABILITY_IAM --stack-name $(STACK) --template-body file://$(CF_YAML_FILENAME) --parameters $(STACK_PARAMS)

.PHONY: update-stack-vpc
update-stack-vpc: CF_YAML_FILENAME=./vpc.yaml
update-stack-vpc:
	$(MAKE) update-stack


# Describe
.PHONY: describe-stack
describe-stack:
	@aws --region=$(AWS_REGION) cloudformation describe-stacks --stack-name $(STACK)

.PHONY: describe-stack-outputs
describe-stack-outputs:
	@aws --region=$(AWS_REGION) cloudformation describe-stacks --stack-name $(STACK) --query 'Stacks[].Outputs[]'


# Delete
.PHONY: delete-stack
delete-stack:
	aws --region=$(AWS_REGION) cloudformation delete-stack --stack-name $(STACK)