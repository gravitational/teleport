resource "teleport_crown_jewel" "example" {
  version = "v1"
  metadata = {
    name        = "example"
    description = "Example crown jewel"
    labels = {
      foo = "bar"
    }
  }
  spec = {
    query = "resource.kind == \"node\""
    teleport_matchers = [{
      kinds = ["node"]
      names = ["my-node"]
      labels = [{
        name   = "env"
        values = ["prod"]
      }]
    }]
    aws_matchers = [{
      types   = ["ec2"]
      regions = ["us-east-1"]
      tags = [{
        key    = "env"
        values = ["prod"]
      }]
    }]
  }
}
