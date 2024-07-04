output "cluster_arn" {
  value = aws_docdb_cluster.this.arn
}

output "discovery_iam_role_arn" {
  value = try(module.iam_discovery[0].iam_role_arn, "")
}

output "access_iam_role_arn" {
  value = try(module.iam_access[0].iam_role_arn, "")
}

output "database_user_iam_role_arn" {
  value = try(module.iam_database_user[0].iam_role_arn, "")
}

output "sample_teleport_discovery_config" {
  value = local.sample_teleport_discovery_config
}

output "create_iam_user_instruction" {
  value = <<-EOT
# Run the following from a machine that can access DocumentDB.

$ wget https://truststore.pki.rds.amazonaws.com/global/global-bundle.pem

$ mongosh --tls --host ${aws_docdb_cluster.this.endpoint}:27017 --tlsCAFile global-bundle.pem  --username ${var.master_username} --password
Enter password: ****************
...

# Once in mongosh. Create an MongoDB user with IAM auth enabled. This example
# gives the user root permissions so adjust accordingly.
use $external;
db.createUser(
    {
        user: "${local.iam_database_user_arn_or_sample}",
        mechanisms: ["MONGODB-AWS"],
        roles: [ { role: "root", db: "admin" },  ]
    }
);

EOT
}
