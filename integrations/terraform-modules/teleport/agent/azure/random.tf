resource "random_string" "name_suffix" {
  count = var.create ? 1 : 0

  length  = 4
  lower   = true
  numeric = true
  special = false
  upper   = false
}
