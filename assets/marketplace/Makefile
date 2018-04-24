

# VPC ID used for builds
BUILD_VPC_ID ?=

# VPC subnet used for builds
BUILD_SUBNET_ID ?=

# Default build region
AWS_REGION ?= us-west-2

# Teleport version
TELEPORT_VERSION ?= 2.6.0-alpha.8

# Teleport UID is a UID of a non-privileged user ID of a teleport
TELEPORT_UID ?= 1007

# Instance type is a single value, sorry
INSTANCE_TYPE ?= t2.micro

# Use comma-separated values without spaces for multiple regions
# For now, limit AMI to regions with DynamoDB encryption at rest enabled
# https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/EncryptionAtRest.html
DESTINATION_REGIONS ?= us-west-2,us-east-1,us-east-2,eu-west-1

# Cloudformation stack name to create, e.g. test1
STACK ?=

# Stack parameters, e.g ParameterKey=KeyName,ParameterValue=KeyName ParameterKey=DomainName,ParameterValue=teleport.example.com ParameterKey=DomainAdminEmail,ParameterValue=admin@example.com ParameterKey=HostedZoneID,ParameterValue=AWSZONEID
STACK_PARAMS ?=

# License S3 URI for enterprise builds, e.g. s3://example.com/path/to/license
TELEPORT_LICENSE_URI ?=
export

.PHONY: oss
oss: TELEPORT_TYPE=oss
oss:
	@echo "Building image $(TELEPORT_VERSION) $(TELEPORT_TYPE)"
	packer build -force template.json

.PHONY: ent
ent: TELEPORT_TYPE=ent
ent: check-ent-vars
	aws s3 cp $(TELEPORT_LICENSE_URI) files/system/license.pem
	@echo "Building image $(TELEPORT_VERSION) $(TELEPORT_TYPE)"
	packer build -force template.json

.PHONY: validate-template
validate-template:
	aws cloudformation validate-template --template-body file://./oss.yaml

.PHONY: create-stack
create-stack:
	$(MAKE) validate-template
	aws --region=$(AWS_REGION) cloudformation create-stack --capabilities CAPABILITY_IAM --stack-name $(STACK) --template-body file://./oss.yaml --parameters $(STACK_PARAMS) 

.PHONY: update-stack
update-stack:
	$(MAKE) validate-template
	aws --region=$(AWS_REGION) cloudformation update-stack --capabilities CAPABILITY_IAM --stack-name $(STACK) --template-body file://./oss.yaml --parameters $(STACK_PARAMS)

.PHONY: describe-stack
describe-stack:
	aws --region=$(AWS_REGION) cloudformation describe-stacks --stack-name $(STACK)

.PHONY: delete-stack
delete-stack:
	aws --region=$(AWS_REGION) cloudformation delete-stack --stack-name $(STACK)

.PHONY: check-ent-vars
check-ent-vars:
	@if [ -z "$(TELEPORT_VERSION)" ]; then \
	  echo "TELEPORT_VERSION is not set"; exit 1; \
	fi;
	@if [ -z "$(TELEPORT_LICENSE_URI)" ]; then \
	  echo "TELEPORT_LICENSE_URI is not set"; exit 1; \
	fi;
