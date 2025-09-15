# Generate the access-graph YAML configuration
locals {
  access_graph_yaml = <<-EOT
backend:
  postgres:
    connection: postgres://postgres:localpass@db:5432/postgres?sslmode=disable

tls:
  cert: certs/server.crt
  key: certs/server.key

log:
  level: DEBUG
  preset: dev

registration_cas:
  - certs/teleport_host_ca.pem
  - certs/internal-ca.crt
EOT
}
