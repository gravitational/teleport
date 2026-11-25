# The provider reads credentials from the environment, for example the
# OKTA_API_TOKEN environment variable.
provider "okta" {
  org_name = var.okta_org_name
  base_url = var.okta_base_url
}
