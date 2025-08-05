module "ecr_teleport" {
  source = "git@github.com:loadsmart/terraform-modules.git//aws-ecr"

  project = "platform/teleport"
  squad   = local.squad

  #allowed_pull_account_ids = ["OD_DEV_ACCOUNT_ID", "OD_PROD_ACCOUNT_ID", "DEV_ACCOUNT_ID",""]
}
