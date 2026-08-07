provider "aws" {
  region = "us-east-1"

  default_tags {
    tags = {
      env                    = "example"
      "teleport.dev/creator" = "gavin.frazar@goteleport.com"
    }
  }
}

provider "teleport" {
  addr         = var.teleport_proxy_addr
  profile_name = replace(var.teleport_proxy_addr, "/:[0-9]+.*/", "")
}
