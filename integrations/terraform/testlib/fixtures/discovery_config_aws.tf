resource "teleport_discovery_config" "aws_example" {
  header = {
    metadata = {
      name = "aws-discovery"
    }
    version = "v1"
  }

  spec = {
    discovery_group = "aws-test"

    aws = [{
      types   = ["ec2"]
      regions = ["us-west-2"]
    }]
  }
}
