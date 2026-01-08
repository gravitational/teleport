# Terraform Access Graph and Teleport

This repo contains Terraform modules for Teleport and Access Graph
infrastructure and deployment locally.

## Prerequisites

Prerequistes for local deployment:

- [`terraform`](https://www.terraform.io/downloads.html)
- [`mkcert`](https://github.com/FiloSottile/mkcert)
- [`docker`](https://docs.docker.com/get-docker/)

## How to create a new deployment

In an empty directory create a `main.tf` file with this boilerplate, updating
all-caps placeholders:

```hcl
module "access_graph_with_teleport" {
  source = "github.com/gravitational/teleport//examples/access-graph/modules/access-graph-with-teleport?depth=1"
  name   = "DEPLOYMENT_NAME"

  local_deployment = {
    target_dir = "OUTPUT"
  }

  teleport = {
    image            = "public.ecr.aws/gravitational/teleport-ent-distroless-debug:18"
    license_pem_path = "PATH/TO/license.pem"
  }

  access_graph = {
    image = "public.ecr.aws/gravitational/access-graph:1.28.1"
    # Uncomment to enable Identity Activity Center
    # identity_activity_center = {
    #   geoip_db_path = "PATH/TO/geolite2-city.mmdb" # optional
    # }
  }
}

terraform {
  required_version = ">= 1.12.1"
  required_providers {
    local = {
      source  = "hashicorp/local"
      version = ">= 2.5.0"
    }
  }
}

output "instructions" {
  value = module.access_graph_with_teleport.instructions
}
```

Then run `terraform init` and `terraform apply` to create the infrastructure.
On success, follow the printed instructions for next steps.

See [`root-sample/main.tf`](root-sample/main.tf) for a fully documented example with
all options.


## Start Teleport and Access Graph

After `terraform apply` has completed successfully, this stack is most easily
run with Docker Compose

    cd OUTPUT
    docker compose up -d

Create your first Teleport user with

    docker compose exec teleport tctl users add --roles=access,editor --logins=root USERNAME

Clean up with

    docker compose down
    terraform destroy

## Run with a Local Teleport binary

If you want to run Teleport via a local binary, for instance when there is no
OCI image available, you can use the same configs and certs.

Start Teleport from the `OUTPUT/teleport` directory:

    cd OUTPUT/teleport
    teleport start -c config.yaml

**Note:** You must have run `docker compose up` at least once to generate
`OUTPUT/access-graph/certs/teleport_host_ca.pem`. Alternatively can retrieve the
CA certificate manually after starting Teleport

    cd OUTPUT
    curl -fsSL \
        --cacert teleport/certs/rootCA.pem \
        --output access-graph/certs/teleport_host_ca.pem \
        'https://root-sample.local:443/webapi/auth/export?type=tls-host'

Start Access Graph and its database with Docker Compose:

    docker compose up --no-deps db access-graph
