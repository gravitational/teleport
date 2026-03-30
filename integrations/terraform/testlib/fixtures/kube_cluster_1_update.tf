resource "teleport_kube_cluster" "test" {
  version = "v3"
  metadata = {
    name    = "test"
    expires = "2032-10-12T07:20:50Z"
    labels = {
      example               = "yes"
      "teleport.dev/origin" = "dynamic"
    }
  }

  spec = {
    kubeconfig = file("./fixtures/kubeconfig-1.yaml")
  }
}
