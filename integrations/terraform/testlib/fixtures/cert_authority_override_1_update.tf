resource "teleport_cert_authority_override" "test" {
  version  = "v1"
  sub_kind = "db_client"
  metadata = {
    name        = "%s"
    description = "updated by terraform test"
  }
  spec = {
    certificate_overrides = [
      {
        certificate = <<EOT
%s
EOT
        chain = [<<EOT
%s
EOT
        ]
        disabled = true
      },
    ]
  }
}
