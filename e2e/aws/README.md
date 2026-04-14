# AWS E2E Database Tests

End-to-end tests for Teleport database access on AWS. The tests spin up a full Teleport
cluster locally that discovers and connects to real AWS databases via IAM authentication.
Database admin users are provisioned automatically by the test code at runtime.

## How It Works

```
Your AWS credentials
  |-- assumes --> Discovery Role ----> discovers databases via AWS APIs
  |-- assumes --> Access Role -------> IAM auth to connect through Teleport
  |-- directly -> DescribeDBInstances  (looks up master secret ARN)
  |-- directly -> GetSecretValue       (fetches master password)
  |                 \-> provisions admin users (CREATE USER, GRANT rds_iam, etc.)
  |                 \-> Teleport auto-provisions ephemeral test users
  |
  \-- (Redshift) Access Role --> DB User Role --> GetClusterCredentialsWithIAM
```

## Setup

Use the `databases-ci` module from `cloud-terraform` to create the databases, VPC,
security groups, and IAM roles. Create a new directory and add a `main.tf`:

```hcl
locals {
  name_prefix = "<your-name>-db-e2e"
}

provider "aws" {
  region = "us-west-2"
}

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

data "http" "my_ip" {
  url = "https://checkip.amazonaws.com"
}

module "databases" {
  source = "/path/to/cloud-terraform/aws/modules/databases-ci"

  name_prefix                      = local.name_prefix
  role_trust_policy_principal_arns = [data.aws_caller_identity.current.arn]
  public_access_ip_ranges          = ["${chomp(data.http.my_ip.response_body)}/32"]
}

# The databases-ci module creates discovery/access roles but does NOT grant the
# caller permissions to describe instances or read master passwords from Secrets
# Manager. The test code needs both to provision database admin users at startup.
# In CI, these are added separately to the GitHub Actions role.
#
# If you have admin access in your AWS account, you already have these permissions
# and can skip this. Documented here for reference.
resource "aws_iam_policy" "e2e_caller" {
  name = "${local.name_prefix}-caller"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid    = "DescribeInstances"
        Effect = "Allow"
        Action = ["rds:DescribeDBInstances", "redshift:DescribeClusters"]
        Resource = [
          module.databases.rds.postgres_instance_arn,
          module.databases.rds.mysql_instance_arn,
          module.databases.rds.mariadb_instance_arn,
          module.databases.redshift_cluster.cluster_arn,
        ]
      },
      {
        Sid    = "ReadMasterPasswords"
        Effect = "Allow"
        Action = "secretsmanager:GetSecretValue"
        Resource = [
          module.databases.rds.postgres_master_user_secret_arn,
          module.databases.rds.mysql_master_user_secret_arn,
          module.databases.rds.mariadb_master_user_secret_arn,
          module.databases.redshift_cluster.master_user_secret_arn,
        ]
      },
    ]
  })
}

output "e2e_env" {
  sensitive = true
  value = {
    TEST_AWS_DB = "true"
    AWS_REGION  = data.aws_region.current.name

    RDS_POSTGRES_INSTANCE_NAME = module.databases.rds.postgres_instance_name
    RDS_MYSQL_INSTANCE_NAME    = module.databases.rds.mysql_instance_name
    RDS_MARIADB_INSTANCE_NAME  = module.databases.rds.mariadb_instance_name
    RDS_ACCESS_ROLE            = module.databases.iam_rds.access_role_arn
    RDS_DISCOVERY_ROLE         = module.databases.iam_rds.discovery_role_arn

    REDSHIFT_CLUSTER_NAME   = module.databases.redshift_cluster.cluster_identifier
    REDSHIFT_ACCESS_ROLE    = module.databases.iam_redshift_cluster.access_role_arn
    REDSHIFT_DISCOVERY_ROLE = module.databases.iam_redshift_cluster.discovery_role_arn
    REDSHIFT_IAM_DB_USER    = module.databases.iam_redshift_cluster.db_user_role_arn

    REDSHIFT_SERVERLESS_WORKGROUP_NAME = module.databases.redshift_serverless.workgroup_name
    REDSHIFT_SERVERLESS_ACCESS_ROLE    = module.databases.iam_redshift_serverless.access_role_arn
    REDSHIFT_SERVERLESS_DISCOVERY_ROLE = module.databases.iam_redshift_serverless.discovery_role_arn
    REDSHIFT_SERVERLESS_IAM_DB_USER    = module.databases.iam_redshift_serverless.db_user_role_arn
    # AWS generates the endpoint name as: <workgroup-name>-rss-access-<region>-<account>
    REDSHIFT_SERVERLESS_ENDPOINT_NAME = "${local.name_prefix}-redshift-serverless-workgroup-rss-access-${data.aws_region.current.name}-${data.aws_caller_identity.current.account_id}"
  }
}
```

Apply:

```bash
terraform init && terraform apply
```

Verify your AWS identity before applying:

```bash
aws sts get-caller-identity
```

Export the environment variables:

```bash
eval "$(terraform output -json e2e_env | jq -r 'to_entries[] | "export \(.key)=\(.value)"')"
```

## Running

See `main_test.go` for the full variable list. Redshift variables are optional — those
tests skip if unset.

```bash
go test -v -race ./e2e/aws/... -timeout 10m

# Subset runs:
go test -v -race ./e2e/aws/... -timeout 10m -run TestDatabases/RDS
go test -v -race ./e2e/aws/... -timeout 10m -run TestDatabases/Redshift
```
