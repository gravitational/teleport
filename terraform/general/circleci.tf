module "circleci" {
  source = "git@github.com:loadsmart/terraform-modules.git//circleci-app"

  project = local.project

  allow_aws_access = true

  providers = {
    aws.main = aws
    aws.dev  = aws.dev
  }
}

data "aws_iam_policy_document" "ecr_push" {
  statement {
    sid = "AllowPushToECR"

    actions = [
      "ecr:InitiateLayerUpload",
      "ecr:UploadLayerPart",
      "ecr:CompleteLayerUpload",
      "ecr:PutImage",
    ]

    resources = [
      module.ecr_teleport.arn,
      "${module.ecr_teleport.arn}/*",
    ]
  }
}

resource "aws_iam_policy" "ecr_push" {
  name   = "circleci-teleport-ECRPusher"
  policy = data.aws_iam_policy_document.ecr_push.json
}

resource "aws_iam_user_policy_attachment" "ecr_push" {
  user       = module.circleci.user_name
  policy_arn = aws_iam_policy.ecr_push.arn
}

resource "aws_iam_user_policy_attachment" "ecr_readonly" {
  user       = module.circleci.user_name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
}
