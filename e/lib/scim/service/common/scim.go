package common

const (
	// UsernameAttribute is the attribute name for the username.
	UsernameAttribute = "userName"
	// GroupsAttribute is the attribute name for the groups.
	GroupsAttribute = "groups"

	// ResourceTypeUser is the resource type for a user.
	ResourceTypeUser = "User"
	// ResourceTypeGroup is the resource type for a group.
	ResourceTypeGroup = "Group"

	// GroupNameAttribute is the attribute name for the group name.
	GroupNameAttribute = "groupName"
	// GroupDisplayNameAttribute is the attribute name for the group display name.
	GroupDisplayNameAttribute = "displayName"

	// ExternalIDLabel is the label used to store the external ID of a resource.
	ExternalIDLabel = "scim/external_id"
)

const (
	// SchemaGroupCore is the SCIM core schema URN for the Group resource type.
	// Defined in RFC 7643, it identifies the schema that describes group-related attributes and behaviors.
	// Reference: https://datatracker.ietf.org/doc/html/rfc7643
	SchemaGroupCore = "urn:ietf:params:scim:schemas:core:2.0:Group"
	// SchemaUserCore is the SCIM core schema URN for the User resource type.
	// It defines the standard attributes and structure for representing user identities in SCIM.
	// Reference: https://datatracker.ietf.org/doc/html/rfc7643
	SchemaUserCore = "urn:ietf:params:scim:schemas:core:2.0:User"
	// SchemaResourceTypeCore is the SCIM core schema URN for the ResourceType resource.
	// This schema describes metadata about available SCIM resource types and their schemas.
	// Reference: https://datatracker.ietf.org/doc/html/rfc7643
	SchemaResourceTypeCore = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"
	// SchemaServiceProviderConfigCore is the SCIM core schema URN for the ServiceProviderConfig resource.
	// It defines the schema used to describe a SCIM service provider’s capabilities and configuration.
	// Reference: https://datatracker.ietf.org/doc/html/rfc7643
	SchemaServiceProviderConfigCore = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
)
