data "aws_caller_identity" "this" {
  count = var.create ? 1 : 0
}

data "aws_region" "this" {
  count = var.create ? 1 : 0
}
