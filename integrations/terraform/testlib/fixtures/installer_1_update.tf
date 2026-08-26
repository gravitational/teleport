resource "teleport_installer" "test" {
  version = "v1"
  metadata = {
    name = "test"
    labels = {
      example = "yes"
    }
  }

  spec = {
    script = <<EOF
[Updated Install Teleport Script]
EOF
  }
}
