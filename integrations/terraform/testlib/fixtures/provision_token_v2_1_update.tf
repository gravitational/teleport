resource "teleport_provision_token" "test2" {
  version = "v2"
  metadata = {
    name    = "test2"
    expires = "2038-01-01T00:00:00Z"
    labels = {
      example = "yes"
    }
  }
  spec = {
    roles       = ["Node", "Auth"]
    join_method = "iam"
    allow = [
      {
        aws_account = "1234567890"
      },
      {
        aws_account = "1111111111"
    }]
  }
}
