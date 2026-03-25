// Warning: The teleportmwi_kubernetes data source will not function correctly
// when the Teleport cluster is fronted by a L7 load balancer that terminates
// TLS.
data "teleportmwi_kubernetes" "my_cluster" {
  selector = {
    name = "my-k8s-cluster"
  }
  credential_ttl = "1h"
}


// https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs
provider "kubernetes" {
  host                   = data.teleportmwi_kubernetes.my_cluster.output.host
  tls_server_name        = data.teleportmwi_kubernetes.my_cluster.output.tls_server_name
  client_certificate     = data.teleportmwi_kubernetes.my_cluster.output.client_certificate
  client_key             = data.teleportmwi_kubernetes.my_cluster.output.client_key
  cluster_ca_certificate = data.teleportmwi_kubernetes.my_cluster.output.cluster_ca_certificate
}