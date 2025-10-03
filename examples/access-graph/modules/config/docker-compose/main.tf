# Generate the docker-compose YAML configuration
locals {
  # Post-start section (optional) for SSH service - properly indented for teleport service
  post_start_section = var.enable_ssh_service ? (
    <<EOT

    post_start:
      - command:
          ["/busybox/sed", "-i", "s|/sbin/nologin|/busybox/sh|", "/etc/passwd"]
      - command:
          - "/busybox/sh"
          - "-c"
          - "echo PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/busybox > /etc/profile"
EOT
  ) : ""

  ssl_cert_env = "\n      SSL_CERT_FILE: /app/certs/rootCA.pem"

  get_teleport_ca_volumes = <<EOT
      - ./access-graph/certs:/certs
      - ./teleport/certs:/ca
EOT

  # Complete docker-compose configuration YAML
  docker_compose_yaml = <<-EOT
services:
  teleport:
    image: ${var.teleport_image}
    ports:
      - "3025:3025" # SSH
      - "3023:3023" # Proxy
      - "443:443" # Web UI
    volumes:
      - ./teleport/config.yaml:/app/config.yaml
      - ./teleport/certs:/app/certs
      - ./teleport/data:/app/data
    environment:${local.ssl_cert_env}
      TELEPORT_CONFIG_FILE: /app/config.yaml${chomp(local.post_start_section)}
    working_dir: /app
    entrypoint: ["/usr/local/bin/teleport"]
    command: ["start", "-c", "/app/config.yaml"]
    hostname: "${var.teleport_hostname}"

  get-teleport-ca:
    image: curlimages/curl:latest
    user: "0:0" # Run as root user for write access to /certs
    depends_on:
      teleport:
        condition: service_started
    volumes:
${chomp(local.get_teleport_ca_volumes)}
    command:
      - "-fsSL"
      - "--cacert"
      - "/ca/rootCA.pem"
      - "--retry"
      - "100"
      - "--retry-all-errors"
      - "--output"
      - "/certs/teleport_host_ca.pem"
      - "https://${var.teleport_hostname}:443/webapi/auth/export?type=tls-host"

  access-graph:
    image: ${var.access_graph_image}
    depends_on:
      get-teleport-ca:
        condition: service_completed_successfully
      db:
        condition: service_healthy
    ports:
      - "50051:50051"
    volumes:
      - ./access-graph/config.yaml:/app/config.yaml
      - ./access-graph/certs:/app/certs
    command: ["--config", "/app/config.yaml"]

  db:
    image: ${var.postgres_image}
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5
    command: |
      postgres
      -c work_mem='16MB'
      -c ssl=on
      -c ssl_ca_file=/var/lib/postgresql/certs/internal-ca.crt
      -c ssl_cert_file=/var/lib/postgresql/certs/postgres.crt
      -c ssl_key_file=/var/lib/postgresql/certs/postgres.key
    ports:
      - "5432:5432"
    volumes:
      - ./access-graph/certs:/var/lib/postgresql/certs
      - db_data:/var/lib/postgresql/data
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: localpass
      POSTGRES_DB: postgres

volumes:
  db_data:
EOT
}
