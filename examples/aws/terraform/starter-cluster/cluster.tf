// Configuration data for teleport.yaml generation
data "template_file" "node_user_data" {
  template = file("data.tpl")

  vars = {
    region                   = var.region
    cluster_name             = var.cluster_name
    email                    = var.email
    domain_name              = var.route53_domain
    dynamo_table_name        = aws_dynamodb_table.teleport.name
    dynamo_events_table_name = aws_dynamodb_table.teleport_events.name
    locks_table_name         = aws_dynamodb_table.teleport_locks.name
    license_path             = var.license_path
    s3_bucket                = var.s3_bucket_name
    enable_mongodb_listener  = var.enable_mongodb_listener
    enable_mysql_listener    = var.enable_mysql_listener
    enable_postgres_listener = var.enable_postgres_listener
    use_acm                  = var.use_acm
    use_letsencrypt          = var.use_letsencrypt
  }
}

// Auth, node, proxy (aka Teleport Cluster) on single AWS instance
resource "aws_instance" "cluster" {
  key_name                    = var.key_name
  ami                         = data.aws_ami.base.id
  instance_type               = var.cluster_instance_type
  subnet_id                   = tolist(data.aws_subnet_ids.all.ids)[0]
  vpc_security_group_ids      = [aws_security_group.cluster.id]
  associate_public_ip_address = true
  user_data                   = data.template_file.node_user_data.rendered
  iam_instance_profile        = aws_iam_role.cluster.id
}

