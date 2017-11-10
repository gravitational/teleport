variable "table_name" {
  type = "string"
}

variable "autoscale_write_target" {
  type = "string"
  default = 50
}

variable "autoscale_read_target" {
  type = "string"
  default = 50
}

variable "autoscale_min_read_capacity" {
  type = "string"
  default = 5
}

variable "autoscale_max_read_capacity" {
  type = "string"
  default = 100
}

variable "autoscale_min_write_capacity" {
  type = "string"
  default = 5
}

variable "autoscale_max_write_capacity" {
  type = "string"
  default = 100
}
