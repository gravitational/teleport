

# VPC ID used for builds
BUILD_VPC_ID ?=

# VPC subnet used for builds
BUILD_SUBNET_ID ?=

# Default build region
AWS_REGION ?= us-west-2

# Teleport version
TELEPORT_VERSION ?= 2.6.0

# Teleport UID is the UID of a non-privileged 'teleport' user
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

# Generate timestamp for builds
BUILD_TIMESTAMP := $(shell TZ=UTC /bin/date "+%Y%m%d-%H%M%S%Z")

# Jenkins test CloudFormation stack name
JENKINS_TEST_STACK_NAME ?= jenkins-teleport-quickstart-test-stack

# Jenkins Cloudformation parameters path
JENKINS_CLOUDFORMATION_PARAMETERS_PATH ?= ./files/build/jenkins-parameters.json

# YAML filename
CF_YAML_FILENAME ?= ./oss.yaml

# S3 Bucket ID
S3_BUCKET_ID ?=
export

.PHONY: oss
oss: TELEPORT_TYPE=oss
oss:
	@echo "Building image $(TELEPORT_VERSION) $(TELEPORT_TYPE)"
	@echo "BUILD_TIMESTAMP=$(BUILD_TIMESTAMP)"
	packer build -force -var build_timestamp=$(BUILD_TIMESTAMP) template.json
	@echo "$(BUILD_TIMESTAMP)" > files/build/oss_build_timestamp.txt

.PHONY: ent
ent: TELEPORT_TYPE=ent
ent: check-ent-vars
	aws s3 cp $(TELEPORT_LICENSE_URI) files/system/license.pem
	@echo "Building image $(TELEPORT_VERSION) $(TELEPORT_TYPE)"
	@echo "BUILD_TIMESTAMP=$(BUILD_TIMESTAMP)"
	packer build -force -var build_timestamp=$(BUILD_TIMESTAMP) template.json
	@echo "$(BUILD_TIMESTAMP)" > files/build/ent_build_timestamp.txt

.PHONY: update-ami-ids-oss
update-ami-ids-oss:
	files/update-ami-ids.sh oss

.PHONY: update-ami-ids-ent
update-ami-ids-ent:
	files/update-ami-ids.sh ent

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
create-stack-vpc: CF_YAML_FILENAME=./oss-vpc.yaml
create-stack-vpc:
	$(MAKE) create-stack


# Update
.PHONY: update-stack
update-stack:
	$(MAKE) validate-template
	aws --region=$(AWS_REGION) cloudformation update-stack --capabilities CAPABILITY_IAM --stack-name $(STACK) --template-body file://$(CF_YAML_FILENAME) --parameters $(STACK_PARAMS)

.PHONY: update-stack-vpc
update-stack-vpc: CF_YAML_FILENAME=./oss-vpc.yaml
update-stack-vpc:
	$(MAKE) update-stack


# Describe
.PHONY: describe-stack
describe-stack:
	@aws --region=$(AWS_REGION) cloudformation describe-stacks --stack-name $(STACK)

.PHONY: describe-stack-outputs
describe-stack-outputs:
	@aws --region=$(AWS_REGION) cloudformation describe-stacks --stack-name $(STACK) --query 'Stacks[].Outputs[]'

.PHONY: describe-stack-outputs-vpc
describe-stack-outputs-vpc: JENKINS_TEST_STACK_NAME=jenkins-teleport-quickstart-test-stack-vpc
describe-stack-outputs-vpc:
	@aws --region=$(AWS_REGION) cloudformation describe-stacks --stack-name $(STACK) --query 'Stacks[].Outputs[]'


# Delete
.PHONY: delete-stack
delete-stack:
	aws --region=$(AWS_REGION) cloudformation delete-stack --stack-name $(STACK)


# Jenkins helpers
.PHONY: jenkins-create-stack
jenkins-create-stack:
	$(MAKE) validate-template
	aws --region=$(AWS_REGION) cloudformation create-stack --capabilities CAPABILITY_IAM --stack-name $(JENKINS_TEST_STACK_NAME) --template-body file://$(CF_YAML_FILENAME) --parameters file://$(JENKINS_CLOUDFORMATION_PARAMETERS_PATH)

.PHONY: jenkins-create-stack-vpc
jenkins-create-stack-vpc: CF_YAML_FILENAME=./oss-vpc.yaml
jenkins-create-stack-vpc: JENKINS_TEST_STACK_NAME=jenkins-teleport-quickstart-test-stack-vpc
jenkins-create-stack-vpc:
	$(MAKE) jenkins-create-stack

.PHONY: jenkins-wait-for-stack-creation
jenkins-wait-for-stack-creation:
	aws --region=$(AWS_REGION) cloudformation wait stack-create-complete --stack-name $(JENKINS_TEST_STACK_NAME)

.PHONY: jenkins-wait-for-stack-creation-vpc
jenkins-wait-for-stack-creation-vpc: JENKINS_TEST_STACK_NAME=jenkins-teleport-quickstart-test-stack-vpc
jenkins-wait-for-stack-creation-vpc:
	$(MAKE) jenkins-wait-for-stack-creation

.PHONY: jenkins-describe-stack
jenkins-describe-stack:
	@aws --region=$(AWS_REGION) cloudformation describe-stacks --stack-name $(JENKINS_TEST_STACK_NAME)

.PHONY: jenkins-describe-stack-outputs
jenkins-describe-stack-outputs:
	@aws --region=$(AWS_REGION) cloudformation describe-stacks --stack-name $(JENKINS_TEST_STACK_NAME) --query 'Stacks[].Outputs[]'

.PHONY: jenkins-describe-stack-vpc
jenkins-describe-stack-vpc: JENKINS_TEST_STACK_NAME=jenkins-teleport-quickstart-test-stack-vpc
jenkins-describe-stack-vpc:
	$(MAKE) jenkins-describe-stack

.PHONY: jenkins-describe-stack-outputs-vpc
jenkins-describe-stack-outputs-vpc: JENKINS_TEST_STACK_NAME=jenkins-teleport-quickstart-test-stack-vpc
jenkins-describe-stack-outputs-vpc:
	@$(MAKE) -s jenkins-describe-stack-outputs

.PHONY: jenkins-get-s3-bucket-id
jenkins-get-s3-bucket-id:
	@$(MAKE) -s jenkins-describe-stack-outputs | jq -r '.[] | select(.OutputKey == "S3BucketID") | .OutputValue'

.PHONY: jenkins-delete-s3-bucket
jenkins-delete-s3-bucket:
	files/s3-delete-all-object-versions.sh $(S3_BUCKET_ID)
	aws s3api delete-bucket --region=$(AWS_REGION) --bucket $(S3_BUCKET_ID)

.PHONY: jenkins-delete-stack
jenkins-delete-stack:
	aws --region=$(AWS_REGION) cloudformation delete-stack --stack-name $(JENKINS_TEST_STACK_NAME)

.PHONY: jenkins-delete-stack-vpc
jenkins-delete-stack-vpc: JENKINS_TEST_STACK_NAME=jenkins-teleport-quickstart-test-stack-vpc
jenkins-delete-stack-vpc:
	$(MAKE) jenkins-delete-stack

.PHONY: jenkins-wait-for-stack-deletion
jenkins-wait-for-stack-deletion:
	aws --region=$(AWS_REGION) cloudformation wait stack-delete-complete --stack-name $(JENKINS_TEST_STACK_NAME)

.PHONY: jenkins-wait-for-stack-deletion-vpc
jenkins-wait-for-stack-deletion-vpc: JENKINS_TEST_STACK_NAME=jenkins-test-stack-vpc
jenkins-wait-for-stack-deletion-vpc:
	$(MAKE) jenkins-wait-for-stack-deletion

.PHONY: check-ent-vars
check-ent-vars:
	@if [ -z "$(TELEPORT_VERSION)" ]; then \
	  echo "TELEPORT_VERSION is not set"; exit 1; \
	fi;
	@if [ -z "$(TELEPORT_LICENSE_URI)" ]; then \
	  echo "TELEPORT_LICENSE_URI is not set"; exit 1; \
	fi;
