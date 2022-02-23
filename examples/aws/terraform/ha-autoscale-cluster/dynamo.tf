// Dynamodb is used as a backend for auth servers,
// and only auth servers need access to the tables
// all other components are stateless.
resource "aws_dynamodb_table" "teleport" {
  name           = var.cluster_name
  billing_mode   = "PAY_PER_REQUEST"
  hash_key       = "HashKey"
  range_key      = "FullPath"
  server_side_encryption {
    enabled = true
  }

  lifecycle {
    ignore_changes = [
      read_capacity,
      write_capacity,
    ]
  }

  attribute {
    name = "HashKey"
    type = "S"
  }

  attribute {
    name = "FullPath"
    type = "S"
  }

  stream_enabled   = "true"
  stream_view_type = "NEW_IMAGE"

  ttl {
    attribute_name = "Expires"
    enabled        = true
  }

  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Dynamodb events table stores events
resource "aws_dynamodb_table" "teleport_events" {
  name           = "${var.cluster_name}-events"
  read_capacity  = 20
  write_capacity = 20
  hash_key       = "SessionID"
  range_key      = "EventIndex"

  server_side_encryption {
    enabled = true
  }

  global_secondary_index {
    name            = "timesearchV2"
    hash_key        = "CreatedAtDate"
    range_key       = "CreatedAt"
    write_capacity  = 20
    read_capacity   = 20
    projection_type = "ALL"
  }

  lifecycle {
    ignore_changes = all
  }

  attribute {
    name = "SessionID"
    type = "S"
  }

  attribute {
    name = "EventIndex"
    type = "N"
  }

  attribute {
    name = "CreatedAtDate"
    type = "S"
  }

  attribute {
    name = "CreatedAt"
    type = "N"
  }

  ttl {
    attribute_name = "Expires"
    enabled        = true
  }

  tags = {
    TeleportCluster = var.cluster_name
  }
}
