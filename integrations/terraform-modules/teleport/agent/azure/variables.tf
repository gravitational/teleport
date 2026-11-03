################################################################################
# Required variables
################################################################################

variable "location" {
  description = "Azure location for the agent. Example: eastus, westus."
  type        = string
  nullable    = false
}

variable "resource_group_name" {
  description = "Resource group where the resources will be created. Must already exist."
  type        = string
  nullable    = false
}

variable "teleport_proxy_public_addr" {
  description = "Teleport cluster proxy public address `host:port`."
  type        = string
  nullable    = false

  validation {
    condition     = !strcontains(var.teleport_proxy_public_addr, "://")
    error_message = "Must not contain a URL scheme"
  }

  validation {
    condition     = var.teleport_proxy_public_addr == "" || strcontains(var.teleport_proxy_public_addr, ":")
    error_message = "The address must be in the form `host:port`."
  }
}

## Services configuration
variable "kubernetes_service" {
  description = <<EOF
Configuration for running this agent as a Kubernetes Service.
`resource_matcher_labels` is required and contains the label selectors to match and proxy Kubernetes resources.
EOF
  type = object({
    enabled                 = optional(bool, true)
    resource_matcher_labels = optional(map(list(string)), {})
  })
  nullable = false

  validation {
    condition     = !var.kubernetes_service.enabled || (length(var.kubernetes_service.resource_matcher_labels) > 0 && alltrue([for values in var.kubernetes_service.resource_matcher_labels : try(length(values) > 0, false)]))
    error_message = "When the Kubernetes Service is enabled, provide at least one `resource_matcher_labels` entry."
  }
}

################################################################################
# Optional variables
################################################################################

variable "teleport_discovery_group_name" {
  description = "If specified, this is added as part of the `resource_matcher_labels`, to match the resources discovered in that discovery group."
  type        = string
  default     = ""
  nullable    = false
}

variable "apply_azure_tags" {
  type        = map(string)
  description = "Additional Azure tags to apply to all created Azure resources."
  default     = {}
  nullable    = false
}

variable "managed_updates_enabled" {
  default     = true
  description = "Whether to resolve the Teleport version from the configured Managed Updates endpoint when applying this module."
  type        = bool
}

variable "managed_updates_group" {
  default     = "default"
  description = "Update group to query through the v2 Managed Updates endpoint."
  type        = string
}

variable "create" {
  default     = true
  description = "Toggle creation of all resources."
  type        = bool
}

variable "container_group_cpu" {
  description = "The required number of CPU cores of the containers."
  type        = number
  default     = 2
  nullable    = false
}

variable "container_group_memory" {
  description = "The required memory of the containers in GB."
  type        = number
  default     = 4
  nullable    = false
}

variable "container_group_desired_count" {
  description = "Desired number of Teleport container groups to run."
  type        = number
  default     = 2
  nullable    = false
}


variable "container_group_name" {
  description = "Name for the container group where the agent runs."
  type        = string
  default     = "teleport-agent"
  nullable    = false
}

variable "container_group_use_name_prefix" {
  default     = true
  description = "Determines whether `var.container_group_name` is used as a prefix of the container group name. The revision and replica index are always appended to container group names."
  type        = bool
}

variable "subnet_id" {
  description = <<EOD
When setting up access to private AKS clusters, set subnet_id to the resource ID of the subnet which will host the Azure Container Instance.
The subnet must be delegated to Microsoft.ContainerInstance/containerGroups and have access to
- the Teleport Proxy Service (var.teleport_proxy_addr)
- the Teleport image registry at https://public.ecr.aws/
- Azure Resource Manager at https://management.azure.com/ (HTTPS/TCP 443) to read cluster details and fetch credentials
- the target AKS API endpoints (DNS resolution and connectivity)

This module does not provision these networking resources.
When omitted or null, no subnet is specified on the container groups.
EOD
  type        = string
  default     = null

  validation {
    condition     = var.subnet_id == null ? true : trimspace(var.subnet_id) != ""
    error_message = "subnet_id must be a non-empty subnet resource ID, or null to omit network placement."
  }
}

variable "environment_vars" {
  default     = {}
  description = "Environment variables to set on the Teleport Azure Container Instance."
  type        = map(string)
}

variable "teleport_container_image" {
  default     = "public.ecr.aws/gravitational/teleport-ent-distroless"
  description = "Container image used for Teleport Azure Container Instance tasks."
  type        = string
}

variable "teleport_version_override" {
  default     = null
  description = <<EOD
teleport_version_override is intended for development and MUST NOT be used to control the Teleport version in a typical deployment.
This module is designed to run a specific Teleport version.
You will face compatibility issues trying to run a different Teleport version with it.
If you want to run Teleport version `X.Y.Z`, you should adjust the module version to `X.Y.Z` instead.
EOD
  type        = string
  nullable    = true
}

variable "log_analytics_workspace_name" {
  default     = "teleport"
  description = "Name of the Log Analytics workspace for the Teleport containers."
  type        = string
}

variable "log_analytics_workspace_retention_days" {
  description = "Retention of container logs in the created Log Analytics workspace."
  type        = number
  default     = 30
  nullable    = false
}

variable "log_analytics_workspace_use_name_prefix" {
  default     = true
  description = "Determines whether log_analytics_workspace_name is used as a prefix of the Log Analytics workspace name."
  type        = bool
}

variable "managed_identity_role" {
  description = "Custom role for the agent's managed identity. Defaults to permissions for the configured service, scoped to the provider's subscription. Override actions, data_actions, or scopes as needed; set to null to disable role creation and assignments."
  type = object({
    actions      = optional(list(string))
    data_actions = optional(list(string))
    scope        = optional(string)
  })
  default = {}

  validation {
    condition     = try(var.managed_identity_role.scope != "", true)
    error_message = "managed_identity_role.scope must be set when defining a custom role. Omit it to default to the provider's subscription."
  }
}
