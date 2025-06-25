module "ecr_teleport" {
  source = "git@github.com:loadsmart/terraform-modules.git//aws-ecr"

  project = "platform/teleport"
  squad   = local.squad
}
