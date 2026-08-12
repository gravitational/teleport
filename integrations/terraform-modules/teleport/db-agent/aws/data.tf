data "aws_caller_identity" "this" {
  count = var.create ? 1 : 0

  lifecycle {
    precondition {
      condition     = var.teleport_proxy_public_addr != ""
      error_message = "The Teleport proxy public address must not be empty when create is true."
    }
  }
}

data "aws_region" "this" {
  count = var.create ? 1 : 0
}

data "aws_iam_policy_document" "ecs_task_inline_policy" {
  count = var.create && (
    var.ecs_task_role_inline_policy != null ||
    contains(var.database_types_for_default_iam_policy, "rds")
  ) ? 1 : 0

  override_policy_documents = (
    var.ecs_task_role_inline_policy == null
    ? []
    : [var.ecs_task_role_inline_policy]
  )

  dynamic "statement" {
    for_each = (
      contains(var.database_types_for_default_iam_policy, "rds") &&
      var.allow_database_modification
    ) ? [true] : []

    content {
      sid = "RDSAutoEnableIAMAuth"

      actions = [
        "rds:ModifyDBCluster",
        "rds:ModifyDBInstance",
      ]
      effect    = "Allow"
      resources = ["*"]
    }
  }

  dynamic "statement" {
    for_each = contains(var.database_types_for_default_iam_policy, "rds") ? [true] : []

    content {
      sid       = "RDSConnect"
      actions   = ["rds-db:connect"]
      effect    = "Allow"
      resources = ["*"]
    }
  }

  dynamic "statement" {
    for_each = contains(var.database_types_for_default_iam_policy, "rds") ? [true] : []

    content {
      sid = "RDSFetchMetadata"

      actions = [
        "rds:DescribeDBClusters",
        "rds:DescribeDBInstances",
      ]
      effect    = "Allow"
      resources = ["*"]
    }
  }
}
