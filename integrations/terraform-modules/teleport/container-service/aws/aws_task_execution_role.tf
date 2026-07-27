################################################################################
# Execution role
################################################################################

resource "aws_iam_role" "ecs_execution" {
  count = var.create ? 1 : 0

  assume_role_policy = one(data.aws_iam_policy_document.ecs_execution_trust[*].json)
  description        = "Execution role used by the Teleport ECS agent task."
  name_prefix        = "${var.ecs_cluster_name}-exec"
  tags               = var.apply_aws_tags
}

data "aws_iam_policy_document" "ecs_execution_trust" {
  count = var.create ? 1 : 0

  statement {
    actions = ["sts:AssumeRole"]

    effect = "Allow"

    condition {
      test     = "StringEquals"
      values   = [one(data.aws_caller_identity.this[*].account_id)]
      variable = "aws:SourceAccount"
    }

    condition {
      test = "ArnLike"
      values = [
        format(
          "arn:%s:ecs:%s:%s:*",
          one(data.aws_partition.this[*].partition),
          one(data.aws_region.this[*].name),
          one(data.aws_caller_identity.this[*].account_id),
        ),
      ]
      variable = "aws:SourceArn"
    }

    principals {
      identifiers = ["ecs-tasks.amazonaws.com"]
      type        = "Service"
    }
  }
}

resource "aws_iam_role_policy" "ecs_execution" {
  count = var.create ? 1 : 0

  name   = "ecs-execution"
  policy = one(data.aws_iam_policy_document.ecs_execution[*].json)
  role   = one(aws_iam_role.ecs_execution[*].id)
}

data "aws_iam_policy_document" "ecs_execution" {
  count = var.create ? 1 : 0

  statement {
    actions = [
      "logs:CreateLogStream",
      "logs:PutLogEvents",
    ]
    effect = "Allow"
    resources = [
      one(aws_cloudwatch_log_group.this[*].arn),
      "${one(aws_cloudwatch_log_group.this[*].arn)}:*",
    ]
  }
}
