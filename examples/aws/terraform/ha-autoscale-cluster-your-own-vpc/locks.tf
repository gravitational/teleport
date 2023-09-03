// Locks is a dynamodb table used as a distributed lock
// to make sure there is only one auth server doing
// Let's Encrypt certificate renewal.
resource "aws_dynamodb_table" "locks" {
  name           = "${var.cluster_name}-locks"
  read_capacity  = 10
  write_capacity = 10
  hash_key       = "Lock"

  // For demo purposes, CMK isn't necessary
  // tfsec:ignore:aws-dynamodb-table-customer-key
  server_side_encryption {
    enabled = true
  }

  point_in_time_recovery {
    enabled = true
  }

  attribute {
    name = "Lock"
    type = "S"
  }

  ttl {
    attribute_name = "Expires"
    enabled        = true
  }

  tags = {
    TeleportCluster = var.cluster_name
  }
}
