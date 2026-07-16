resource "aws_cloudwatch_log_group" "this" {
  count = var.create ? 1 : 0

  name = var.ecs_task_cloudwatch_log_group_name
  region = coalesce(
    var.ecs_task_cloudwatch_log_group_region,
    one(data.aws_region.this[*].name),
  )
  retention_in_days = var.ecs_task_cloudwatch_log_group_retention_days
  skip_destroy      = var.ecs_task_cloudwatch_log_group_skip_destroy
  tags              = var.apply_aws_tags
}
