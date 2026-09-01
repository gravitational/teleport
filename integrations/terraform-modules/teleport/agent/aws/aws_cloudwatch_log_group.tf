resource "aws_cloudwatch_log_group" "this" {
  count = var.create ? 1 : 0

  name = (
    var.ecs_task_cloudwatch_log_group_use_name_prefix
    ? format("%v-%v",
      var.ecs_task_cloudwatch_log_group_name,
      one(random_string.name_suffix[*].result),
    )
    : var.ecs_task_cloudwatch_log_group_name
  )
  region = coalesce(
    var.ecs_task_cloudwatch_log_group_region,
    one(data.aws_region.this[*].region),
  )
  kms_key_id        = var.ecs_task_cloudwatch_log_group_kms_key_id
  retention_in_days = var.ecs_task_cloudwatch_log_group_retention_days
  skip_destroy      = var.ecs_task_cloudwatch_log_group_skip_destroy
  tags              = var.apply_aws_tags
}
