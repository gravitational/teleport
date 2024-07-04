resource "teleport_server" "test" {
  version  = "v2"
  sub_kind = "openssh-ec2-ice"
  metadata = {
    name = "test"
  }
  spec = {
    addr     = "127.0.0.1:23"
    hostname = "test.local"
    cloud_metadata = {
      aws = {
        account_id  = "123"
        instance_id = "123"
        region      = "us-east-1"
        vpc_id      = "123"
        integration = "foo"
        subnet_id   = "123"
      }
    }
  }
}
