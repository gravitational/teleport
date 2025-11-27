# Teleport Discovery Config
#
# Discovery Config resources define matchers for the Teleport Discovery Service.
# The Discovery Service automatically discovers and enrolls cloud resources
# (EC2 instances, RDS databases, EKS clusters, Azure VMs, etc.) into your
# Teleport cluster.
#
# Each Discovery Config is associated with a discovery_group. Discovery Services
# load matchers from Discovery Configs that share the same discovery_group.

# Example: AWS Discovery Config for EC2 instances and RDS databases
resource "teleport_discovery_config" "aws_example" {
  header = {
    metadata = {
      name        = "aws-discovery"
      description = "Discover AWS EC2 instances and RDS databases"
      labels = {
        env = "production"
      }
    }
    version = "v1"
  }

  spec = {
    discovery_group = "aws-prod"

    aws = [{
      types   = ["ec2", "rds"]
      regions = ["us-west-2", "us-east-1"]
      tags = {
        "env" = ["prod", "production"]
      }
      install_params = {
        join_method = "iam"
        join_token  = "aws-discovery-token"
        script_name = "default-installer"
      }
    }]
  }
}

# Example: Azure Discovery Config for VMs and AKS clusters
resource "teleport_discovery_config" "azure_example" {
  header = {
    metadata = {
      name        = "azure-discovery"
      description = "Discover Azure VMs and AKS clusters"
    }
    version = "v1"
  }

  spec = {
    discovery_group = "azure-prod"

    azure = [{
      types           = ["vm", "aks"]
      regions         = ["eastus", "westus2"]
      subscriptions   = ["00000000-0000-0000-0000-000000000000"]
      resource_groups = ["my-resource-group"]
      tags = {
        "*" = ["*"]
      }
      install_params = {
        join_method = "azure"
        join_token  = "azure-discovery-token"
        script_name = "default-installer"
        azure = {
          client_id = "00000000-0000-0000-0000-000000000000"
        }
      }
    }]
  }
}

