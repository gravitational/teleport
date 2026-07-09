################################################################################
# Task role
################################################################################

resource "aws_iam_role" "ecs_task" {
  count = var.create ? 1 : 0

  assume_role_policy = one(data.aws_iam_policy_document.ecs_task_trust[*].json)
  description        = "Task role used by the Teleport ECS agent task."
  name_prefix        = "${var.ecs_cluster_name}-task"
  tags               = var.apply_aws_tags
}

resource "aws_iam_role_policy" "ecs_task" {
  count = var.create && var.ecs_task_role_inline_policy != null ? 1 : 0

  name   = "ecs-task"
  policy = var.ecs_task_role_inline_policy
  role   = one(aws_iam_role.ecs_task[*].id)
}

data "aws_iam_policy_document" "ecs_task_trust" {
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
