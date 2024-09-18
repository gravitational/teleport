module "iam_database_user" {
  count                   = var.create_database_user_iam_role ? 1 : 0
  source                  = "terraform-aws-modules/iam/aws//modules/iam-assumable-role"
  create_role             = true
  create_instance_profile = true
  role_requires_mfa       = false
  role_name               = "${var.identifier}-teleport-admin-user"
  trusted_role_services   = ["ec2.amazonaws.com", "lambda.amazonaws.com"]
  trusted_role_arns = concat(
    var.databaase_user_iam_role_trusted_role_arns,
    module.iam_access.*.iam_role_arn,
  )
}

module "iam_database_user_lambda" {
  count                   = var.create_database_user_iam_role ? 1 : 0
  source                  = "terraform-aws-modules/iam/aws//modules/iam-assumable-role"
  create_role             = true
  create_instance_profile = false
  role_requires_mfa       = false
  role_name               = "${var.identifier}-teleport-admin-user-lambda"
  trusted_role_services   = ["lambda.amazonaws.com"]
  custom_role_policy_arns = ["arn:aws:iam::aws:policy/service-role/AWSLambdaVPCAccessExecutionRole"]
}

resource "aws_lambda_function" "iam_database_user" {
  count            = var.create_database_user_iam_role ? 1 : 0
  filename         = local.zip_file
  function_name    = "${var.identifier}-create-iam-user"
  role             = module.iam_database_user_lambda[count.index].iam_role_arn
  handler          = "create_iam_user.create_handler"
  source_code_hash = filesha256(local.zip_file)
  runtime          = "python3.12"
  timeout          = 60

  vpc_config {
    subnet_ids         = var.subnet_ids
    security_group_ids = var.security_group_ids
  }

  tags = var.tags
}

resource "aws_lambda_invocation" "iam_database_user" {
  count         = var.create_database_user_iam_role ? 1 : 0
  function_name = aws_lambda_function.iam_database_user[count.index].function_name

  input = jsonencode({
    "DOCDB_ENDPOINT" : aws_docdb_cluster.this.endpoint,
    "DOCDB_MASTER_USERNAME" : var.master_username,
    "DOCDB_MASTER_PASSWORD" : var.master_password,
    "DOCDB_IAM_USER" : module.iam_database_user[count.index].iam_role_arn,
  })
}

locals {
  zip_file                        = "${path.module}/create_iam_user/create_iam_user.zip"
  iam_database_user_arn_or_sample = try(module.iam_database_user[0].iam_role_arn, "arn:aws:iam::123456789012:role/your-database-user-iam-role")
}
