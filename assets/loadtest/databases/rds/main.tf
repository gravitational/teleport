data "aws_caller_identity" "current" {}
data "aws_partition" "current" {}

locals {
  account_id = data.aws_caller_identity.current.account_id
  partition  = data.aws_partition.current.partition

  iam_role_name   = "${var.prefix}-database-access"
  iam_policy_name = "${var.prefix}-database-access"
  iam_role_arn    = "arn:${local.partition}:iam::${local.account_id}:role/${local.iam_role_name}"

  pg_proxy_name    = "${var.prefix}-postgres-proxy"
  mysql_proxy_name = "${var.prefix}-mysql-proxy"
}

provider "aws" {
  region = var.region
}

data "aws_eks_cluster" "cluster" {
  name = var.eks_cluster_name
}

data "aws_vpc" "selected" {
  id = data.aws_eks_cluster.cluster.vpc_config[0].vpc_id
}

data "aws_subnets" "selected" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.selected.id]
  }

  # Always create the databases on subnet that have public access since we need
  # to access them locally to create the database users.
  filter {
    name   = "tag:Name"
    values = ["*Public*"]
  }
}

module "postgres_security_group" {
  source  = "terraform-aws-modules/security-group/aws"
  version = "~> 5.1"

  name            = var.prefix
  use_name_prefix = true
  description     = "Load test security group for PostgreSQL"
  vpc_id          = data.aws_vpc.selected.id

  # This is required when deploying with RDS Proxy (as it access Secrets Manager)
  egress_rules = ["all-all"]
  ingress_with_cidr_blocks = [
    {
      from_port   = var.postgres_port
      to_port     = var.postgres_port
      protocol    = "tcp"
      description = "PostgreSQL access from within VPC"
      cidr_blocks = data.aws_vpc.selected.cidr_block
    }
  ]
}

module "mysql_security_group" {
  source  = "terraform-aws-modules/security-group/aws"
  version = "~> 5.1"

  name            = var.prefix
  use_name_prefix = true
  description     = "Load test security group for MySQL"
  vpc_id          = data.aws_vpc.selected.id

  # This is required when deploying with RDS Proxy (as it access Secrets Manager)
  egress_rules = ["all-all"]
  ingress_with_cidr_blocks = [
    {
      from_port   = var.mysql_port
      to_port     = var.mysql_port
      protocol    = "tcp"
      description = "MySQL access from within VPC"
      cidr_blocks = data.aws_vpc.selected.cidr_block
    }
  ]
}

module "subnet_group" {
  source  = "terraform-aws-modules/rds/aws//modules/db_subnet_group"
  version = "~> 6.1"

  use_name_prefix = true
  name            = var.prefix
  subnet_ids      = sort(data.aws_subnets.selected.ids)
}

module "pg" {
  source  = "terraform-aws-modules/rds/aws"
  version = "~> 6.1"

  create_db_instance             = var.create_postgres
  identifier                     = var.prefix
  instance_use_identifier_prefix = true

  # All available versions: https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/CHAP_PostgreSQL.html#PostgreSQL.Concepts
  engine                = "postgres"
  engine_version        = "14"
  family                = "postgres14"
  major_engine_version  = "14"
  instance_class        = var.instance_class
  allocated_storage     = 20
  max_allocated_storage = 100

  db_name                     = var.database_name
  username                    = var.database_master_username
  port                        = var.postgres_port
  manage_master_user_password = true
  ca_cert_identifier          = "rds-ca-rsa2048-g1"

  vpc_security_group_ids = [module.postgres_security_group.security_group_id]
  db_subnet_group_name   = module.subnet_group.db_subnet_group_id

  create_db_subnet_group              = false
  publicly_accessible                 = false
  iam_database_authentication_enabled = false
  multi_az                            = false
  create_cloudwatch_log_group         = false
  skip_final_snapshot                 = true
  deletion_protection                 = false
  performance_insights_enabled        = false
  create_monitoring_role              = false
  create_db_option_group              = false
  create_db_parameter_group           = false
}

module "mysql" {
  source  = "terraform-aws-modules/rds/aws"
  version = "~> 6.1"

  create_db_instance             = var.create_mysql
  identifier                     = var.prefix
  instance_use_identifier_prefix = true

  # All available versions: http://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/CHAP_MySQL.html#MySQL.Concepts.VersionMgmt
  engine                = "mysql"
  engine_version        = "8.0"
  family                = "mysql8.0"
  major_engine_version  = "8.0"
  instance_class        = var.instance_class
  allocated_storage     = 20
  max_allocated_storage = 100

  db_name                     = var.database_name
  username                    = var.database_master_username
  port                        = var.mysql_port
  manage_master_user_password = true
  ca_cert_identifier          = "rds-ca-rsa2048-g1"

  vpc_security_group_ids = [module.mysql_security_group.security_group_id]
  db_subnet_group_name   = module.subnet_group.db_subnet_group_id

  create_db_subnet_group              = false
  publicly_accessible                 = false
  iam_database_authentication_enabled = false
  multi_az                            = false
  create_cloudwatch_log_group         = false
  skip_final_snapshot                 = true
  deletion_protection                 = false
  performance_insights_enabled        = false
  create_monitoring_role              = false
  create_db_option_group              = false
  create_db_parameter_group           = false
}

module "database_agent_policy" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-policy"
  version = "~> 5.28"

  name        = local.iam_policy_name
  path        = "/"
  description = "Teleport load test policy for acessing RDS databases."

  policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "rds:DescribeDBProxies",
                "rds:DescribeDBProxyEndpoints",
                "rds:DescribeDBProxyTargets",
                "rds:ListTagsForResource",
                "rds-db:connect"
            ],
            "Resource": "*"
        }
    ]
}
EOF
}

module "iam_eks_role" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-eks-role"
  version = "~> 5.28"

  role_name = local.iam_role_name
  role_policy_arns = {
    policy = module.database_agent_policy.arn
  }
  cluster_service_accounts = {
    "${var.eks_cluster_name}" = ["${var.database_access_namespace}:${var.database_access_svc_account_name}"]
  }
}

module "postgres_proxy" {
  source  = "terraform-aws-modules/rds-proxy/aws"
  version = "~> 3"

  create                 = var.create_postgres
  name                   = local.pg_proxy_name
  vpc_subnet_ids         = data.aws_subnets.selected.ids
  vpc_security_group_ids = [module.postgres_security_group.security_group_id]

  auth = {
    "master_user" = {
      description = "${var.database_master_username} master user"
      secret_arn  = module.pg.db_instance_master_user_secret_arn
      iam_auth    = "REQUIRED"
    }
  }

  engine_family          = "POSTGRESQL"
  debug_logging          = false
  target_db_instance     = true
  db_instance_identifier = module.pg.db_instance_identifier

  tags = {
    "loadtest" = var.prefix
  }
}

module "mysql_proxy" {
  source  = "terraform-aws-modules/rds-proxy/aws"
  version = "~> 3"

  create                 = var.create_mysql
  name                   = local.mysql_proxy_name
  vpc_subnet_ids         = data.aws_subnets.selected.ids
  vpc_security_group_ids = [module.mysql_security_group.security_group_id]

  auth = {
    "master_user" = {
      description = "${var.database_master_username} master user"
      secret_arn  = module.mysql.db_instance_master_user_secret_arn
      iam_auth    = "REQUIRED"
    }
  }

  engine_family          = "MYSQL"
  debug_logging          = false
  target_db_instance     = true
  db_instance_identifier = module.mysql.db_instance_identifier

  tags = {
    "loadtest" = var.prefix
  }
}
