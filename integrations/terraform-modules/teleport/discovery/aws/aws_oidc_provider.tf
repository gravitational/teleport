################################################################################
# AWS IAM OIDC Provider
################################################################################

locals {
  aws_iam_oidc_provider_arn = try(
    aws_iam_openid_connect_provider.teleport[0].arn,
    "arn:${local.aws_partition}:iam::${local.aws_account_id}:oidc-provider/${local.aws_iam_oidc_provider_name}",
  )
  aws_iam_oidc_provider_aud  = "discover.teleport"
  aws_iam_oidc_provider_name = trimprefix(local.aws_iam_oidc_provider_url, "https://")
  # strip the port since AWS OIDC provider doesn't support port in the url
  aws_iam_oidc_provider_url = replace(local.teleport_proxy_public_url, "/:[0-9]+.*/", "")
  create_aws_iam_openid_connect_provider = (
    local.create
    && local.use_oidc_integration
    && var.create_aws_iam_openid_connect_provider
  )
  use_oidc_integration = var.discovery_service_iam_credential_source.use_oidc_integration
}

data "tls_certificate" "teleport_proxy" {
  count = local.create_aws_iam_openid_connect_provider ? 1 : 0

  url = local.teleport_proxy_public_url
}

# Create an AWS OIDC Provider, so that the Teleport Discovery Service can use
# OIDC to assume the discovery AWS IAM role.
resource "aws_iam_openid_connect_provider" "teleport" {
  count = local.create_aws_iam_openid_connect_provider ? 1 : 0

  url             = local.aws_iam_oidc_provider_url
  client_id_list  = [local.aws_iam_oidc_provider_aud]
  thumbprint_list = [data.tls_certificate.teleport_proxy[0].certificates[0].sha1_fingerprint]
  tags            = local.apply_aws_tags
}
