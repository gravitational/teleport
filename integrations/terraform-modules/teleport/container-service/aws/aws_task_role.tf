################################################################################
# Task role
################################################################################

locals {
  enable_ecs_task_inline_policy = var.create && (
    var.ecs_task_role_self_assumption_allowed ||
    var.ecs_task_role_inline_policy != null
  )
  ecs_task_role_name = format("%v-%v-task-%v",
    one(aws_ecs_cluster.teleport_agent[*].name),
    one(data.aws_region.this[*].region),
    one(random_string.name_suffix[*].result),
  )
  ecs_task_role_arn = format(
    "arn:%s:iam::%s:role/%s",
    one(data.aws_partition.this[*].partition),
    one(data.aws_caller_identity.this[*].account_id),
    local.ecs_task_role_name,
  )
}

resource "aws_iam_role" "ecs_task" {
  count = var.create ? 1 : 0

  assume_role_policy = one(data.aws_iam_policy_document.ecs_task_trust[*].json)
  description        = "Task role used by the Teleport ECS agent task."
  name               = local.ecs_task_role_name
  tags               = var.apply_aws_tags
}

resource "aws_iam_role_policy" "ecs_task" {
  count = local.enable_ecs_task_inline_policy ? 1 : 0

  name   = "ecs-task"
  policy = one(data.aws_iam_policy_document.ecs_task_inline_policy[*].json)
  role   = one(aws_iam_role.ecs_task[*].id)
}

data "aws_iam_policy_document" "ecs_task_inline_policy" {
  count = local.enable_ecs_task_inline_policy ? 1 : 0

  source_policy_documents = (
    var.ecs_task_role_inline_policy == null
    ? []
    : [var.ecs_task_role_inline_policy]
  )

  dynamic "statement" {
    for_each = var.ecs_task_role_self_assumption_allowed ? [true] : []

    content {
      actions   = ["sts:AssumeRole"]
      effect    = "Allow"
      resources = [local.ecs_task_role_arn]
    }
  }
}

data "aws_iam_policy_document" "ecs_task_trust" {
  count = var.create ? 1 : 0

  statement {
    sid     = "TrustECS"
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
          one(data.aws_region.this[*].region),
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

  dynamic "statement" {
    for_each = var.ecs_task_role_self_assumption_allowed ? [true] : []

    content {
      sid     = "TrustSelf"
      actions = ["sts:AssumeRole"]
      effect  = "Allow"

      condition {
        test     = "ArnEquals"
        values   = [local.ecs_task_role_arn]
        variable = "aws:PrincipalArn"
      }

      principals {
        identifiers = [format(
          "arn:%s:iam::%s:root",
          one(data.aws_partition.this[*].partition),
          one(data.aws_caller_identity.this[*].account_id),
        )]
        type = "AWS"
      }
    }
  }
}
