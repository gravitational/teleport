package generic

import (
	"context"
	"encoding/json"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/structpb"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
)

// Attribute defines a single attribute of a SCIM resource schema.
// It corresponds to the "attributes" array items in the SCIM schema definition
// as described in RFC 7643, Section 7.2.
//
// Reference: https://datatracker.ietf.org/doc/html/rfc7643#section-7.2
type Attribute struct {
	// Name is the attribute's name. This MUST be unique within the schema.
	Name string `json:"name"`
	// Type represents the data type of the attribute (e.g., "string", "boolean", "complex").
	// See RFC 7643 Section 7.2 for allowed types.
	Type string `json:"type"`
	// MultiValued indicates whether the attribute is an array (true) or a single value (false).
	MultiValued bool `json:"multiValued"`
	// Description provides a human-readable explanation of the attribute's purpose.
	Description string `json:"description"`
	// Required indicates whether the attribute must be present in the resource.
	Required bool `json:"required"`
	// CaseExact specifies whether the attribute value should be treated as case-sensitive
	// when performing comparisons.
	CaseExact bool `json:"caseExact,omitempty"`
	// Mutability defines whether the attribute can be read, written, or both.
	// Allowed values: "readOnly", "readWrite", "immutable", "writeOnly".
	Mutability string `json:"mutability,omitempty"`
	// Returned defines how the attribute is returned in responses.
	// Allowed values: "always", "never", "default", "request".
	Returned string `json:"returned,omitempty"`
	// Uniqueness indicates how unique the attribute's value must be across resources.
	// Allowed values: "none", "server", "global".
	Uniqueness string `json:"uniqueness,omitempty"`
	// SubAttributes defines the structure of a complex attribute if Type is "complex".
	// Each sub-attribute is itself an Attribute definition.
	SubAttributes []Attribute `json:"subAttributes,omitempty"`
}

// Schema defines the structure of a SCIM resource, including its attributes.
// This corresponds to the Schema Resource in RFC 7643, Section 7.
//
// Reference: https://datatracker.ietf.org/doc/html/rfc7643#section-7
type Schema struct {
	// Name is the name of the resource schema (e.g., "User" or "Group").
	Name string `json:"name"`
	// Description is a human-readable summary of the schema's purpose.
	Description string `json:"description"`
	// Attributes is the list of attributes that define the structure of the resource.
	Attributes []Attribute `json:"attributes"`
}

// resourceTypeDesc defines the common interface for SCIM resource type metadata.
type resourceTypeDesc interface {
	getResource() (*scimpb.Resource, error)
	getName() string
	getEndpoint() string
	getSchema() string
}

// groupResourceType represents the SCIM Group resource type metadata.
type groupResourceType struct{}

func (groupResourceType) getName() string     { return "Group" }
func (groupResourceType) getEndpoint() string { return "/Groups" }
func (groupResourceType) getSchema() string   { return common.SchemaGroupCore }

func (g groupResourceType) getResource() (*scimpb.Resource, error) {
	return buildResourceType(g.getName(), g.getEndpoint(), g.getSchema(), "/Group")
}

// userResourceType represents the SCIM User resource type metadata.
type userResourceType struct{}

func (userResourceType) getName() string     { return "User" }
func (userResourceType) getEndpoint() string { return "/Users" }
func (userResourceType) getSchema() string   { return common.SchemaUserCore }

func (u userResourceType) getResource() (*scimpb.Resource, error) {
	return buildResourceType(u.getName(), u.getEndpoint(), u.getSchema(), "/User")
}

// buildResourceType constructs an SCIM ResourceType object.
func buildResourceType(name, endpoint, schema, location string) (*scimpb.Resource, error) {
	attrs := map[string]any{
		"name":     name,
		"endpoint": endpoint,
		"schema":   schema,
	}
	structAttrs, err := structpb.NewStruct(attrs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &scimpb.Resource{
		Schemas: []string{common.SchemaResourceTypeCore},
		Id:      name,
		Meta: &scimpb.Meta{
			ResourceType: "ResourceType",
			Location:     location,
		},
		Attributes: structAttrs,
	}, nil
}

// serviceProviderConfigHandler returns static configuration describing SCIM provider capabilities.
type serviceProviderConfigHandler struct {
	notImplementedHandler
}

// ServiceProviderConfigAttribute contains the supported operations and features of this SCIM provider.
var ServiceProviderConfigAttribute = map[string]any{
	"patch":          map[string]any{"supported": false},
	"bulk":           map[string]any{"supported": false, "maxOperations": 0, "maxPayloadSize": 0},
	"filter":         map[string]any{"supported": true, "maxResults": 200},
	"changePassword": map[string]any{"supported": false},
	"sort":           map[string]any{"supported": false},
	"etag":           map[string]any{"supported": false},
	"authenticationSchemes": []any{
		map[string]any{"type": "oauthbearertoken", "name": "OAuth Bearer Token"},
	},
}

// GetResource implements SCIM GET for service provider configuration.
func (serviceProviderConfigHandler) GetResource(ctx context.Context, req *scimpb.GetSCIMResourceRequest) (*scimpb.Resource, error) {
	resource, err := structpb.NewStruct(ServiceProviderConfigAttribute)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &scimpb.Resource{
		Schemas: []string{common.SchemaServiceProviderConfigCore},
		Meta: &scimpb.Meta{
			ResourceType: "ServiceProviderConfig",
			Location:     "/ServiceProviderConfig",
		},
		Attributes: resource,
	}, nil
}

// schemaHandler provides SCIM schema definitions for User and Group resources.
type schemaHandler struct {
	common.Config
	Plugin *types.PluginV1
	notImplementedHandler
}

// GetResource returns schema information for a requested SCIM resource type.
func (h schemaHandler) GetResource(ctx context.Context, req *scimpb.GetSCIMResourceRequest) (*scimpb.Resource, error) {
	switch req.GetTarget().GetResourceId() {
	case common.SchemaGroupCore:
		return buildGroupSchemaResource(), nil
	case common.SchemaUserCore:
		return buildUserSchemaResource(), nil
	default:
		return nil, trace.BadParameter("unsupported resource type: %v", req.GetTarget().GetResourceId())
	}
}

// ListResources returns all supported SCIM schema resources.
func (h schemaHandler) ListResources(ctx context.Context, req *scimpb.ListSCIMResourcesRequest) (*scimpb.ResourceList, error) {
	resources := []*scimpb.Resource{
		buildUserSchemaResource(),
		buildGroupSchemaResource(),
	}
	return &scimpb.ResourceList{
		TotalResults: int32(len(resources)),
		ItemsPerPage: int32(len(resources)),
		StartIndex:   1,
		Resources:    resources,
	}, nil
}

// GroupAttribute is the group resource SCIM schema definition.
var GroupAttribute = Schema{
	Name:        "Group",
	Description: "Group Schema",
	Attributes: []Attribute{
		{
			Name:        "displayName",
			Type:        "string",
			MultiValued: false,
			Description: "A human-readable name for the Group. REQUIRED.",
			Required:    true,
			CaseExact:   false,
			Mutability:  "readWrite",
			Returned:    "default",
			Uniqueness:  "none",
		},
		{
			Name:        "members",
			Type:        "complex",
			MultiValued: true,
			Description: "A list of members of the Group.",
			Required:    false,
			Mutability:  "readWrite",
			Returned:    "default",
			SubAttributes: []Attribute{
				{
					Name:        "value",
					Type:        "string",
					Description: "Identifier of the member of this Group.",
					Required:    false,
					Mutability:  "readWrite",
					Returned:    "default",
					CaseExact:   false,
					Uniqueness:  "none",
					MultiValued: false,
				},
			},
		},
	},
}

func buildGroupSchemaResource() *scimpb.Resource {
	groupAttrs, err := structToPB(GroupAttribute)
	if err != nil {
		return nil
	}
	return &scimpb.Resource{
		Id:      common.SchemaGroupCore,
		Schemas: []string{common.SchemaGroupCore},
		Meta: &scimpb.Meta{
			ResourceType: "Schema",
			Location:     "/Schemas/" + common.SchemaGroupCore,
		},
		Attributes: groupAttrs,
	}
}

// UserAttribute is the user resource SCIM schema definition.
var UserAttribute = Schema{
	Name:        "User",
	Description: "User Schema",
	Attributes: []Attribute{
		{
			Name:        "userName",
			Type:        "string",
			MultiValued: false,
			Description: "Unique identifier for the User. REQUIRED.",
			Required:    true,
			CaseExact:   false,
			Mutability:  "readWrite",
			Returned:    "default",
			Uniqueness:  "server",
		},
		{
			Name:        "active",
			Type:        "boolean",
			MultiValued: false,
			Description: "Indicates the user's active status.",
			Required:    false,
			Mutability:  "readWrite",
			Returned:    "default",
		},
		{
			Name:        "groups",
			Type:        "complex",
			MultiValued: true,
			Description: "Groups the user belongs to.",
			Required:    false,
			Returned:    "request",
			Mutability:  "readOnly",
			SubAttributes: []Attribute{
				{
					Name:        "value",
					Type:        "string",
					Description: "Identifier of the group.",
					Required:    false,
					Returned:    "request",
					Mutability:  "readOnly",
					CaseExact:   false,
					Uniqueness:  "none",
					MultiValued: false,
				},
			},
		},
	},
}

// buildUserSchemaResource builds the SCIM schema resource for Users.
func buildUserSchemaResource() *scimpb.Resource {
	userAttrs, err := structToPB(UserAttribute)
	if err != nil {
		return nil
	}
	return &scimpb.Resource{
		Id:      common.SchemaUserCore,
		Schemas: []string{common.SchemaUserCore},
		Meta: &scimpb.Meta{
			ResourceType: "Schema",
			Location:     "/Schemas/" + common.SchemaUserCore,
		},
		Attributes: userAttrs,
	}
}

// resourceTypesHandler provides a list of SCIM resource types (User, Group).
type resourceTypesHandler struct {
	common.Config
	Plugin *types.PluginV1
	notImplementedHandler
}

// ListResources returns the available SCIM resource types.
func (h resourceTypesHandler) ListResources(ctx context.Context, req *scimpb.ListSCIMResourcesRequest) (*scimpb.ResourceList, error) {
	resourceTypes := []resourceTypeDesc{
		userResourceType{},
		groupResourceType{},
	}

	var resources []*scimpb.Resource
	for _, r := range resourceTypes {
		res, err := r.getResource()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, res)
	}

	return &scimpb.ResourceList{
		TotalResults: int32(len(resources)),
		ItemsPerPage: int32(len(resources)),
		StartIndex:   1,
		Resources:    resources,
	}, nil
}

// notImplementedHandler provides default stub methods for unsupported SCIM operations.
// It needs to fulfill the SCIM CRUD interface for the discovery resource that
// support only List or Get operations.
type notImplementedHandler struct{}

func (notImplementedHandler) GetResource(context.Context, *scimpb.GetSCIMResourceRequest) (*scimpb.Resource, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (notImplementedHandler) CreateResource(context.Context, *scimpb.CreateSCIMResourceRequest) (*scimpb.Resource, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (notImplementedHandler) ListResources(context.Context, *scimpb.ListSCIMResourcesRequest) (*scimpb.ResourceList, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (notImplementedHandler) UpdateResource(context.Context, *scimpb.UpdateSCIMResourceRequest) (*scimpb.Resource, error) {
	return nil, trace.NotImplemented("not implemented")
}

func (notImplementedHandler) DeleteResource(context.Context, *scimpb.DeleteSCIMResourceRequest) error {
	return trace.NotImplemented("not implemented")
}

// structToPB converts a Go struct to a Protocol Buffers Struct.
func structToPB(s any) (*structpb.Struct, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, trace.Wrap(err)
	}

	return structpb.NewStruct(m)
}
