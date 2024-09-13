variable "env_label" {
  description = "\"env\" label value of resources that a user with this role can access"
  type        = string
}
variable "principals" {
  description = "Map of strings to lists of strings corresponding to the principals on infrastructure resources that the user can access. Examples: logins, kubernetes_groups, db_names."
  type        = map(list(string))
}

variable "request_roles" {
  description = "List of strings indicating the names of roles that a user with this role can request"
  type        = list(string)
}
