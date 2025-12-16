variable "access_list_name" {
  description = "Name of the access list to create"
  type        = string
}

variable "access_list_title" {
  description = "Human-readable title for the access list"
  type        = string
}

variable "access_list_description" {
  description = "Description of the access list"
  type        = string
  default     = ""
}

variable "preset_type" {
  description = "Type of access list preset: 'long-term' (members get direct access) or 'short-term' (members get requester role for on-demand access)"
  type        = string
  validation {
    condition     = contains(["long-term", "short-term"], var.preset_type)
    error_message = "preset_type must be either 'long-term' or 'short-term'"
  }
}

variable "access_roles" {
  description = "List of existing role names to use in the access list. These roles must already exist in Teleport."
  type        = list(string)
}

variable "audit" {
  description = "Audit configuration for the access list"
  type = object({
    next_audit_date = string
    recurrence = optional(object({
      frequency  = string
      day_of_month = optional(number)
    }))
  })
  default = null
}

variable "membership_requires" {
  description = "Conditions that must be met for membership"
  type = object({
    roles = optional(list(string))
    traits = optional(map(list(string)))
  })
  default = null
}

variable "ownership_requires" {
  description = "Conditions that must be met for ownership"
  type = object({
    roles = optional(list(string))
    traits = optional(map(list(string)))
  })
  default = null
}

variable "members" {
  description = "Members to add to the access list. Can be users (membership_kind='user', default) or nested access lists (membership_kind='list')"
  type = list(object({
    name            = string
    membership_kind = optional(string, "user") # "user" (default) or "list" (nested access list)
    joined          = optional(string)
    expires         = optional(string)
    reason          = optional(string)
    added_by        = optional(string)
  }))
  default = []

  validation {
    condition = alltrue([
      for member in var.members :
      contains(["user", "list"], member.membership_kind)
    ])
    error_message = "membership_kind must be either 'user' or 'list'"
  }
}

variable "owners" {
  description = "Owners configuration for the access list spec"
  type = list(object({
    name        = string
    description = optional(string)
  }))
  default = []
}
