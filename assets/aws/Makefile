# Teleport version
# This must be a _released_ version of Teleport, i.e. one which has binaries
# available for download on https://goteleport.com/download
# Unreleased versions will fail to build.
TELEPORT_VERSION ?= 15.3.1

# Teleport UID is the UID of a non-privileged 'teleport' user
TELEPORT_UID ?= 1007

# Use comma-separated values without spaces for multiple regions
# For now, limit AMI publishing to non opt-in regions
# https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html
# You can get this with $(AWS_DEFAULT_REGION=us-west-2 aws ec2 describe-regions | jq -r '.Regions | map(.RegionName) | join(",")')
DESTINATION_REGIONS ?= ap-northeast-1,ap-northeast-2,ap-northeast-3,ap-south-1,ap-southeast-1,ap-southeast-2,ca-central-1,eu-central-1,eu-north-1,eu-west-1,eu-west-2,eu-west-3,sa-east-1,us-east-1,us-east-2,us-west-1,us-west-2

# Generate timestamp for builds
BUILD_TIMESTAMP := $(shell TZ=UTC /bin/date "+%Y%m%d-%H%M%S%Z")

# AWS account ID which hosts the public Teleport AMIs
AWS_ACCOUNT_ID ?= 146628656107
export


# Other helpers
.PHONY: check-vars
check-vars:
	@if [ -z "$(TELEPORT_VERSION)" ]; then \
	  echo "TELEPORT_VERSION is not set"; exit 1; \
	fi;

.PHONY: update-ami-ids-terraform
update-ami-ids-terraform:
	@echo -e "\nUpdating OSS Terraform image IDs"
	go run ./cmd/update-ami-id --aws-account $(AWS_ACCOUNT_ID) --regions $(DESTINATION_REGIONS) --version $(TELEPORT_VERSION) --type oss
	@echo -e "\nUpdating Enterprise Terraform image IDs"
	go run ./cmd/update-ami-id --aws-account $(AWS_ACCOUNT_ID) --regions $(DESTINATION_REGIONS) --version $(TELEPORT_VERSION) --type ent
	@echo -e "\nUpdating Enterprise FIPS Terraform image IDs"
	go run ./cmd/update-ami-id --aws-account $(AWS_ACCOUNT_ID) --regions $(DESTINATION_REGIONS) --version $(TELEPORT_VERSION) --type ent-fips

# you will need the GitHub 'gh' CLI installed and working to be able to use this target
# https://github.com/cli/cli/releases/latest
AUTO_BRANCH_NAME := "ami-auto-branch-$(shell date +%s)"
MAKEFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
.PHONY: create-update-pr
create-update-pr: update-ami-ids-terraform
	@echo "Creating PR for updates"
	sed -E "s/^TELEPORT_VERSION \?= [0-9.]+$$/TELEPORT_VERSION ?= $(TELEPORT_VERSION)/g" $(MAKEFILE_PATH) > Makefile.tmp
	mv Makefile.tmp $(MAKEFILE_PATH)
	git add -A ../../examples/aws $(shell pwd)
	git checkout -b $(AUTO_BRANCH_NAME)
	git commit -am "[auto] Update AMI IDs for $(TELEPORT_VERSION)"
	git push --set-upstream origin $(AUTO_BRANCH_NAME)
	gh pr create --fill --label=automated --label=terraform --label=no-changelog $(if $(AMI_PR_REVIEWER),--reviewer=$(AMI_PR_REVIEWER))
