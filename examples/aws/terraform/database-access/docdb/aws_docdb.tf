resource "aws_docdb_subnet_group" "this" {
  name       = var.identifier
  subnet_ids = var.subnet_ids
  tags       = var.tags
}

resource "aws_docdb_cluster" "this" {
  cluster_identifier              = var.identifier
  engine                          = "docdb"
  engine_version                  = var.engine_version
  master_username                 = var.master_username
  master_password                 = var.master_password
  skip_final_snapshot             = true
  db_subnet_group_name            = aws_docdb_subnet_group.this.name
  db_cluster_parameter_group_name = var.parameter_group_name
  vpc_security_group_ids          = var.security_group_ids
  tags                            = var.tags
}

resource "aws_docdb_cluster_instance" "this" {
  count              = var.num_instances
  identifier_prefix  = var.identifier
  cluster_identifier = aws_docdb_cluster.this.id
  instance_class     = var.instance_class
  tags               = var.tags
}
