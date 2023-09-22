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
