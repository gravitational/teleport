data "aws_availability_zones" "this" {
  for_each = local.regions

  region = each.key
}
