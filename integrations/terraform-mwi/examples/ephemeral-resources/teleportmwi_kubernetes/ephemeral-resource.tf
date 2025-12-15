// Warning: The teleportmwi_kubernetes ephemeral resource will not function
// correctly when the Teleport cluster is fronted by a L7 load balancer that
// terminates TLS.
ephemeral "teleportmwi_kubernetes" "my_cluster" {
  selector = {
    name = "my-k8s-cluster"
  }
  credential_ttl = "1h"
}


// https://registry.terraform.io/providers/hashicorp/kubernetes/latest/docs
provider "kubernetes" {
  host                   = ephemeral.teleportmwi_kubernetes.my_cluster.output.host
  tls_server_name        = ephemeral.teleportmwi_kubernetes.my_cluster.output.tls_server_name
  client_certificate     = ephemeral.teleportmwi_kubernetes.my_cluster.output.client_certificate
  client_key             = ephemeral.teleportmwi_kubernetes.my_cluster.output.client_key
  cluster_ca_certificate = ephemeral.teleportmwi_kubernetes.my_cluster.output.cluster_ca_certificate
}