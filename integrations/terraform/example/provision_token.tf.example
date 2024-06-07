# Teleport Provision Token resource

resource "teleport_provision_token" "example" {
  metadata = {
    expires = "2022-10-12T07:20:51Z"
    description = "Example token"

    labels = {
      example = "yes" 
      "teleport.dev/origin" = "dynamic" // This label is added on Teleport side by default
    }
  }

  spec = {
    roles = ["Node", "Auth"]
  }
}

resource "teleport_provision_token" "iam-token" {
  metadata = {
    name = "iam-token"
  }
  spec = {
    roles       = ["Bot"]
    bot_name    = "mybot"
    join_method = "iam"
    allow = [{
      aws_account = "123456789012"
    }]
  }
}
