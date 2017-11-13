provider "aws" {
  region     = "${var.region}"
}

variable "region" {
  type = "string"
}

variable "table_name" {
  type = "string"
}

module dynamoautoscale {
 source = "../../../modules/dynamoautoscale"
 table_name = "${var.table_name}"
}