variable "project" {
  type        = string
  description = "The project to manage resources in."
}

variable "region" {
  type        = string
  description = "The region to manage resources in."
  default     = "us-central1"
}

variable "zone" {
  type        = string
  description = "The zone to manage resources in."
  default     = "us-central1-a"
}

variable "nodes_per_zone" {
  type        = number
  description = "The number of kubernetes nodes per zone"
  default     = 1
}

variable "cluster_name" {
  type        = string
  description = "The name of the cluster, unique within the project and location."
  default     = "loadtest"
}

variable "node_locations" {
  type        = set(string)
  description = "The list of zones in which the node pool's nodes should be located."
  default     = ["us-central1-b", "us-central1-f", "us-central1-c"]
}

variable "network" {
  type        = string
  description = "The network to be used by the cluster"
  default     = "default-us-central1"
}