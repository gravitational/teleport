# VPC ID used for builds
BUILD_VPC_ID ?=

# VPC subnet used for builds
BUILD_SUBNET_ID ?=

# Override for AMI name if needed
BUILD_AMI_NAME ?=

# Default build region
AWS_REGION ?= us-west-2

# Teleport version
TELEPORT_VERSION ?= 3.1.1

# Teleport UID is the UID of a non-privileged 'teleport' user
TELEPORT_UID ?= 1007

# Instance type is a single value, sorry
INSTANCE_TYPE ?= t2.micro

# Use comma-separated values without spaces for multiple regions
# For now, limit AMI to regions with DynamoDB encryption at rest enabled
# https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/EncryptionAtRest.html
DESTINATION_REGIONS ?= us-west-2,us-east-1,us-east-2,eu-west-1

# Generate timestamp for builds
BUILD_TIMESTAMP := $(shell TZ=UTC /bin/date "+%Y%m%d-%H%M%S%Z")

# S3 Bucket ID
S3_BUCKET_ID ?=

# Telegraf version
TELEGRAF_VERSION ?= 1.9.3

# InfluxDB version
INFLUXDB_VERSION ?= 1.7.1

# Grafana version
GRAFANA_VERSION ?= 5.4.3
export


.PHONY: oss
oss: TELEPORT_TYPE=oss
oss: check-vars
oss:
	@echo "Building image $(TELEPORT_VERSION) $(TELEPORT_TYPE)"
	@echo "BUILD_TIMESTAMP=$(BUILD_TIMESTAMP)"
	packer build -force -var build_timestamp=$(BUILD_TIMESTAMP) single-ami.json

.PHONY: oss-jenkins-build
oss-jenkins-build: TELEPORT_TYPE=oss
oss-jenkins-build: check-vars
oss-jenkins-build:
	@echo "Building image $(TELEPORT_VERSION) $(TELEPORT_TYPE) with name $(BUILD_AMI_NAME)"
	packer build -force -var ami_name=$(BUILD_AMI_NAME) single-ami.json

.PHONY: ent
ent: TELEPORT_TYPE=ent
ent: check-vars
	@echo "Building image $(TELEPORT_VERSION) $(TELEPORT_TYPE)"
	@echo "BUILD_TIMESTAMP=$(BUILD_TIMESTAMP)"
	packer build -force -var build_timestamp=$(BUILD_TIMESTAMP) single-ami.json

.PHONY: ent-jenkins-build
ent-jenkins-build: TELEPORT_TYPE=ent
ent-jenkins-build: check-vars
ent-jenkins-build:
	@echo "Building image $(TELEPORT_VERSION) $(TELEPORT_TYPE) with name $(BUILD_AMI_NAME)"
	packer build -force -var ami_name=$(BUILD_AMI_NAME) single-ami.json


# Other helpers
.PHONY: check-vars
check-vars:
	@if [ -z "$(TELEPORT_VERSION)" ]; then \
	  echo "TELEPORT_VERSION is not set"; exit 1; \
	fi;
