resource "teleport_provision_token" "iam-token" {
  version = "v2"
  metadata = {
    name = "iam-token"
  }
  spec = {
    roles       = ["Node"]
    join_method = "iam"
    allow = [{
      aws_account = "123456789012"
    }]
  }
}

