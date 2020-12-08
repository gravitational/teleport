# These are default values which shouldn't need to be changed by users

variable "influxdb_version" {
  type    = string
  default = "1.4.2"
}

variable "telegraf_version" {
  type    = string
  default = "1.5.1-1"
}

variable "grafana_version" {
  type    = string
  default = "4.6.3"
}

# Assign a number to each AZ letter used in our configuration
variable "az_number" {
  default = {
    a = 1
    b = 2
    c = 3
    d = 4
    e = 5
    f = 6
    g = 7
    h = 8
  }
}

# Assign a number to each different subnet type that we use
# This helps avoid conflicts across different availability zones
variable "az_subnet_type" {
  default = {
    bastion = 1
    auth = 2
    proxy = 3
    node = 4
    monitor = 5
  }
}
