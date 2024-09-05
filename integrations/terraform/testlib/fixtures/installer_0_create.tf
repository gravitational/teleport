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
[Install Teleport Script]
EOF
  }
}
