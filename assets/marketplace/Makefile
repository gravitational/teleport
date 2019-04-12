# VPC ID used for builds
BUILD_VPC_ID ?=

# VPC subnet used for builds
BUILD_SUBNET_ID ?=

# Public AMI name
PUBLIC_AMI_NAME ?=

# Marketplace AMI name ?=
MARKETPLACE_AMI_NAME ?=

# Default build region
AWS_REGION ?= us-west-2

# Teleport version
TELEPORT_VERSION ?= 3.1.7

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


# Build local 'debug' AMI
.PHONY: oss
oss: TELEPORT_TYPE=oss
oss: check-vars
oss:
	@echo "Building image $(TELEPORT_VERSION) $(TELEPORT_TYPE)"
	@echo "BUILD_TIMESTAMP=$(BUILD_TIMESTAMP)"
	mkdir -p files/build
	packer build -force -var build_timestamp=$(BUILD_TIMESTAMP) -except teleport-aws-linux-marketplace single-ami.json
	@echo "$(BUILD_TIMESTAMP)" > files/build/oss_build_timestamp.txt

# Build named 'production' AMI and marketplace version
.PHONY: oss-jenkins-build
oss-jenkins-build: TELEPORT_TYPE=oss
oss-jenkins-build: check-vars
oss-jenkins-build:
	@echo "Building image $(TELEPORT_VERSION) $(TELEPORT_TYPE) via Jenkins"
	@echo "Public AMI name: $(PUBLIC_AMI_NAME)"
	@echo "Marketplace AMI name: $(MARKETPLACE_AMI_NAME)"
	@echo "BUILD_TIMESTAMP=$(BUILD_TIMESTAMP)"
	mkdir -p files/build
	packer build -force -var ami_name=$(PUBLIC_AMI_NAME) -var marketplace_ami_name=$(MARKETPLACE_AMI_NAME) -var build_type=production -var build_timestamp=$(BUILD_TIMESTAMP) single-ami.json
	@echo "$(BUILD_TIMESTAMP)" > files/build/oss_build_timestamp.txt

.PHONY: change-amis-to-public-oss
change-amis-to-public-oss:
	@echo "Making OSS AMIs public"
	bash files/make-amis-public.sh oss

# Build local 'debug' AMI
.PHONY: ent
ent: TELEPORT_TYPE=ent
ent: check-vars
	@echo "Building image $(TELEPORT_VERSION) $(TELEPORT_TYPE)"
	@echo "BUILD_TIMESTAMP=$(BUILD_TIMESTAMP)"
	mkdir -p files/build
	packer build -force -var build_timestamp=$(BUILD_TIMESTAMP) -except teleport-aws-linux-marketplace single-ami.json
	@echo "$(BUILD_TIMESTAMP)" > files/build/ent_build_timestamp.txt

# Build named 'production' AMI and marketplace version
.PHONY: ent-jenkins-build
ent-jenkins-build: TELEPORT_TYPE=ent
ent-jenkins-build: check-vars
ent-jenkins-build:
	@echo "Building image $(TELEPORT_VERSION) $(TELEPORT_TYPE) via Jenkins"
	@echo "Public AMI name: $(PUBLIC_AMI_NAME)"
	@echo "Marketplace AMI name: $(MARKETPLACE_AMI_NAME)"
	@echo "BUILD_TIMESTAMP=$(BUILD_TIMESTAMP)"
	mkdir -p files/build
	packer build -force -var ami_name=$(PUBLIC_AMI_NAME) -var marketplace_ami_name=$(MARKETPLACE_AMI_NAME) -var build_type=production -var build_timestamp=$(BUILD_TIMESTAMP) single-ami.json
	@echo "$(BUILD_TIMESTAMP)" > files/build/ent_build_timestamp.txt

.PHONY: change-amis-to-public-ent
change-amis-to-public-ent:
	@echo "Making Enterprise AMIs public"
	bash files/make-amis-public.sh ent


# Other helpers
.PHONY: check-vars
check-vars:
	@if [ -z "$(TELEPORT_VERSION)" ]; then \
	  echo "TELEPORT_VERSION is not set"; exit 1; \
	fi;
