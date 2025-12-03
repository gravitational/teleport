# Generate the Teleport YAML configuration
locals {
  https_keypairs_section = <<EOT
  https_keypairs:
    - key_file: certs/teleport-key.pem
      cert_file: certs/teleport.pem
EOT

  # SSH service enabled value
  ssh_enabled = var.enable_ssh_service ? "yes" : "no"

  # Access graph enabled
  access_graph_enabled = var.enable_access_graph ? "true" : "false"

  # Audit log enabled
  audit_log_enabled = var.enable_audit_log ? "true" : "false"

  # Complete Teleport configuration YAML
  teleport_yaml = <<-EOT
version: v3
teleport:
  nodename: ${var.nodename}
  data_dir: data
  log:
    severity: DEBUG
    format:
      output: text
      extra_fields: [level, timestamp, component, caller]

auth_service:
  listen_addr: 0.0.0.0:3025
  web_idle_timeout: 12h
  license_file: ../certs/license.pem # relative to data directory
  proxy_listener_mode: multiplex
  authentication:
    type: local
    second_factor: on
    webauthn:
      rp_id: ${var.address}

proxy_service:
  enabled: "yes"
  public_addr: ${var.address}
  web_listen_addr: 0.0.0.0:443
${local.https_keypairs_section}
ssh_service:
  enabled: "${local.ssh_enabled}"
  listen_addr: 0.0.0.0:3022
  public_addr: ${var.address}:3022
  commands:
    - name: hostname
      command: [hostname]
      period: 1m0s

access_graph:
  enabled: ${local.access_graph_enabled}
  endpoint: access-graph:50051
  ca: certs/internal-ca.crt
  audit_log:
    enabled: ${local.audit_log_enabled}

debug_service:
  enabled: false

# Other services
kubernetes_service:
  enabled: "no"
  listen_addr: tagdev.teleport:3026
  public_addr: tagdev.teleport
  kubeconfig_file: c/kubeconfig

app_service:
  enabled: "no"
  resources:
    - labels:
        "*": "*"

db_service:
  enabled: "no"
  resources:
    - labels:
        "*": "*"
EOT
}
