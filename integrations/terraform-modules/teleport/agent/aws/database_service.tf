data "aws_iam_policy_document" "database_task_inline_policy" {
  count = local.database_service_config_enabled && var.create && (
    var.ecs_task_role_inline_policy != null ||
    length(var.database_service.default_iam_policy_database_types) > 0
  ) ? 1 : 0

  override_policy_documents = (
    var.ecs_task_role_inline_policy == null
    ? []
    : [var.ecs_task_role_inline_policy]
  )

  dynamic "statement" {
    for_each = (
      contains(var.database_service.default_iam_policy_database_types, "rds") &&
      var.database_service.allow_database_modification
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
    for_each = contains(var.database_service.default_iam_policy_database_types, "rds") ? [true] : []

    content {
      sid       = "RDSConnect"
      actions   = ["rds-db:connect"]
      effect    = "Allow"
      resources = ["*"]
    }
  }

  dynamic "statement" {
    for_each = contains(var.database_service.default_iam_policy_database_types, "rds") ? [true] : []

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

  dynamic "statement" {
    for_each = contains(var.database_service.default_iam_policy_database_types, "rdsproxy") ? [true] : []

    content {
      # This is the same permission as for RDS, but we document that these
      # policy statements can be overridden by SID, so keep distinct statements
      # for RDS and RDS Proxy.
      sid       = "RDSProxyConnect"
      actions   = ["rds-db:connect"]
      effect    = "Allow"
      resources = ["*"]
    }
  }

  dynamic "statement" {
    for_each = contains(var.database_service.default_iam_policy_database_types, "rdsproxy") ? [true] : []

    content {
      sid = "RDSProxyFetchMetadata"

      actions = [
        "rds:DescribeDBProxies",
        "rds:DescribeDBProxyEndpoints",
      ]
      effect    = "Allow"
      resources = ["*"]
    }
  }
}
