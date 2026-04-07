resource "teleport_kube_cluster" "my-cluster" {
  version = "v3"
  metadata = {
    name = "test"
    labels = {
      example               = "yes"
      "teleport.dev/origin" = "dynamic"
    }
  }

  spec = {
    kubeconfig = file("./my-cluster-kubeconfig.yaml")
  }
}
