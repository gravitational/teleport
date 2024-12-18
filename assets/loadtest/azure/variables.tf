variable "location" {
  type        = string
  nullable    = false
  description = "Azure region of the resource group"
}

variable "cluster_prefix" {
  type        = string
  nullable    = false
  description = "Prefix of the cluster name in the DNS zone"
}

variable "dns_zone" {
  type        = string
  nullable    = false
  description = "DNS zone to put the Teleport cluster in"
}

variable "dns_zone_rg" {
  type        = string
  nullable    = false
  description = "Resource group of the DNS zone"
}

variable "deploy_teleport" {
  type        = bool
  nullable    = false
  description = "Install the Teleport helm release"
}

variable "teleport_version" {
  type        = string
  nullable    = false
  description = "Version of Teleport"
}

variable "subscription_id" {
  type        = string
  nullable    = false
  description = "The Azure subscription_id"
}
