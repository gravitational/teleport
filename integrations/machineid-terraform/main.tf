terraform {
  required_providers {
    machineid = {
      source = "terraform.releases.teleport.dev/gravitational/machineid"
      version = "18.0.0-dev"
    }
  }
}

provider machineid {
  addr = "hugoshaka-internal.cloud.gravitational.io:443"
  join_method = "token"
  join_token = "0fe40e9534db817bf8ffe2bb258712c2"
}

data "machineid_kubernetes_service_v2" "demo-cluster" {
  selectors = [
    {
      name = "demo-cluster"
    }
  ]
}

output "kubeconfig" {
  value = "machineid_kubernetes_service_v2.demo-cluster.output"
}
