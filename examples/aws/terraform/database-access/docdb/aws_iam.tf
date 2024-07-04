module "iam_discovery" {
  count                   = var.create_discovery_iam_role ? 1 : 0
  source                  = "terraform-aws-modules/iam/aws//modules/iam-assumable-role"
  create_role             = true
  create_instance_profile = true
  role_requires_mfa       = false
  role_name               = "${var.identifier}-teleport-discovery"
  trusted_role_services   = ["ec2.amazonaws.com"]
}

resource "aws_iam_role_policy" "iam_discovery" {
  count = var.create_discovery_iam_role ? 1 : 0
  name  = "teleport_policy"
  role  = module.iam_discovery[count.index].iam_role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = [
          "rds:DescribeDBClusters",
        ]
        Effect   = "Allow"
        Resource = "*"
      },
    ]
  })
}

module "iam_access" {
  count                   = var.create_access_iam_role ? 1 : 0
  source                  = "terraform-aws-modules/iam/aws//modules/iam-assumable-role"
  create_role             = true
  create_instance_profile = true
  role_requires_mfa       = false
  role_name               = "${var.identifier}-teleport-access"
  trusted_role_services   = ["ec2.amazonaws.com"]
}

resource "aws_iam_role_policy" "iam_access" {
  count = var.create_access_iam_role ? 1 : 0
  name  = "teleport_policy"
  role  = module.iam_access[count.index].iam_role_name

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        // Required to validate URL and fetch metadata.
        Action = [
          "rds:DescribeDBClusters",
        ]
        Effect   = "Allow"
        Resource = "*"
      },
      // In addition, this role needs to assume db user roles. For simplicity,
      // we are adding this role in the db user's trust policy below. Since
      // they are in the same account, no sts:AssumeRole is required here.
    ]
  })
}

module "iam_database_user" {
  count                   = var.create_database_user_iam_role ? 1 : 0
  source                  = "terraform-aws-modules/iam/aws//modules/iam-assumable-role"
  create_role             = true
  create_instance_profile = true
  role_requires_mfa       = false
  role_name               = "${var.identifier}-teleport-user"
  trusted_role_services   = ["ec2.amazonaws.com"]
  trusted_role_arns = concat(
    var.databaase_user_iam_role_trusted_role_arns,
    module.iam_access.*.iam_role_arn,
  )
}

locals {
  iam_database_user_arn_or_sample = try(module.iam_database_user[0].iam_role_arn, "arn:aws:iam::123456789012:role/your-database-user-iam-role")
}
