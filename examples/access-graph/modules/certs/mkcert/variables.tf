variable "address" {
  description = "The primary address/hostname for the certificate (e.g., tele.local)"
  type        = string
}

variable "target_dir" {
  description = "Directory where certificates will be generated"
  type        = string
}
