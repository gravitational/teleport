# Okta provider org_name/base_url are derived from the okta_org_url variable.
provider "okta" {
  org_name = local.okta_org_name
  base_url = local.okta_base_url
}

# Teleport auth uses local `tsh` login profile.
provider "teleport" {
  addr         = "${var.teleport_domain}:443"
  profile_name = var.teleport_domain
}
