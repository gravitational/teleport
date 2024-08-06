data "aws_region" "current" {}

resource "teleport_database" "cluster_endpoint" {
  count = var.create_teleport_databases ? 1 : 0

  version = "v3"
  metadata = {
    name        = "${var.identifier}-cluster"
    description = "Cluster endpoint for DocumentDB ${var.identifier}"
    labels = merge(
      aws_docdb_cluster.this.tags_all,
      { "teleport.dev/origin" = "dynamic" },
    )
  }

  spec = {
    protocol = "mongodb"
    uri      = "${aws_docdb_cluster.this.endpoint}:27017"
  }
}

resource "teleport_database" "reader_endpoint" {
  count = (var.create_teleport_databases && var.num_instances > 1) ? 1 : 0

  version = "v3"
  metadata = {
    name        = "${var.identifier}-reader"
    description = "Reader endpoint for DocumentDB ${var.identifier}"
    labels = merge(
      aws_docdb_cluster.this.tags_all,
      { "teleport.dev/origin" = "dynamic" },
    )
  }

  spec = {
    protocol = "mongodb"
    uri      = "${aws_docdb_cluster.this.reader_endpoint}:27017"
  }
}

resource "teleport_database" "instance_endpoint" {
  count = var.create_teleport_databases && var.create_teleport_databases_per_instance ? var.num_instances : 0

  version = "v3"
  metadata = {
    name        = "${var.identifier}-instance-${count.index}"
    description = "Instance endpoint for DocumentDB ${var.identifier} instance ${count.index}"
    labels = merge(
      aws_docdb_cluster.this.tags_all,
      { "teleport.dev/origin" = "dynamic" },
    )
  }

  spec = {
    protocol = "mongodb"
    uri      = "${aws_docdb_cluster_instance.this[count.index].endpoint}:${aws_docdb_cluster_instance.this[count.index].port}"
  }
}

locals {
  // TODO consider removing this once discovery_config can be added through our
  // Terraform provider.
  sample_teleport_discovery_config = <<-EOT
discovery_service:
  enabled: "yes"
  discovery_group: "docdb-discovery"
  aws:
    - types: ["docdb"]
      regions: [-1"]
      tags:
        "": ""
EOT
}
