/*
Copyright 2026 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

type testTeleportResource struct {
	Name        string
	Revision    string
	Value       string
	ServerValue string
}

type testResourceClient struct {
	before       *testTeleportResource
	after        *testTeleportResource
	upsertCalled bool
}

func (c *testResourceClient) Get(context.Context, GetResourceRequest[NameIdentifier]) (*testTeleportResource, error) {
	if c.upsertCalled {
		return cloneTestTeleportResource(c.after), nil
	}

	return cloneTestTeleportResource(c.before), nil
}

func (c *testResourceClient) Create(context.Context, *testTeleportResource) error {
	return errors.New("create not implemented")
}

func (c *testResourceClient) Upsert(_ context.Context, resource *testTeleportResource) error {
	c.upsertCalled = true
	return nil
}

func (c *testResourceClient) Delete(context.Context, NameIdentifier) error {
	return errors.New("delete not implemented")
}

func TestResourceTeleportUpdateSetsStateFromRetrievedResource(t *testing.T) {
	schema := testTeleportResourceSchema()
	client := &testResourceClient{
		before: &testTeleportResource{
			Name:        "example",
			Revision:    "1",
			Value:       "old-value",
			ServerValue: "old-server-value",
		},
		after: &testTeleportResource{
			Name:        "example",
			Revision:    "2",
			Value:       "planned-value",
			ServerValue: "server-normalized-value",
		},
	}

	resource := resourceTeleport[testTeleportResource, NameIdentifier]{
		resourceClient: client,
		resource: resourceTeleportType[testTeleportResource, NameIdentifier]{
			kind:          "test",
			fromTerraform: copyTestTeleportResourceFromTerraform,
			toTerraform:   copyTestTeleportResourceToTerraform,
			identifierFromState: func(ctx context.Context, s tfsdk.State) (NameIdentifier, diag.Diagnostics) {
				var id types.String
				diags := s.GetAttribute(ctx, path.Root("name"), &id)
				if diags.HasError() {
					return NameIdentifier{}, diags
				}
				return NameIdentifier{Name: id.Value}, diag.Diagnostics{}
			},
			identifierFromResource: func(ttr *testTeleportResource) NameIdentifier {
				return NameIdentifier{Name: ttr.Name}
			},
			identifierFromImportID: func(s string) (NameIdentifier, error) {
				return NameIdentifier{Name: s}, nil
			},
			resourceRevision: func(resource *testTeleportResource) string {
				return resource.Revision
			},
		},
		provider: &Provider{
			configured: true,
			RetryConfig: RetryConfig{
				Base:     time.Nanosecond,
				Cap:      time.Nanosecond,
				MaxTries: 3,
			},
		},
	}

	resp := &tfsdk.UpdateResourceResponse{
		State: tfsdk.State{Schema: schema},
	}
	resource.Update(t.Context(), tfsdk.UpdateResourceRequest{
		Plan: testTeleportResourcePlan(t, t.Context(), schema, map[string]string{
			"id":           "example",
			"name":         "example",
			"revision":     "1",
			"value":        "planned-value",
			"server_value": "old-server-value",
		}),
	}, resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)

	var serverValue types.String
	diags := resp.State.GetAttribute(t.Context(), path.Root("server_value"), &serverValue)
	require.False(t, diags.HasError(), diags)
	require.Equal(t, "server-normalized-value", serverValue.Value)

	var revision types.String
	diags = resp.State.GetAttribute(t.Context(), path.Root("revision"), &revision)
	require.False(t, diags.HasError(), diags)
	require.Equal(t, "2", revision.Value)
}

func testTeleportResourceSchema() tfsdk.Schema {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.StringType,
				Computed: true,
			},
			"name": {
				Type:     types.StringType,
				Required: true,
			},
			"revision": {
				Type:     types.StringType,
				Computed: true,
			},
			"value": {
				Type:     types.StringType,
				Required: true,
			},
			"server_value": {
				Type:     types.StringType,
				Computed: true,
			},
		},
	}
}

func testTeleportResourcePlan(t *testing.T, ctx context.Context, schema tfsdk.Schema, values map[string]string) tfsdk.Plan {
	t.Helper()

	attrs := make(map[string]attr.Value, len(values))
	attrTypes := make(map[string]attr.Type, len(schema.Attributes))
	for name, attribute := range schema.Attributes {
		attrTypes[name] = attribute.Type
		attrs[name] = types.String{Value: values[name]}
	}

	object := types.Object{
		Attrs:     attrs,
		AttrTypes: attrTypes,
	}
	raw, err := object.ToTerraformValue(ctx)
	require.NoError(t, err)

	return tfsdk.Plan{
		Raw:    raw,
		Schema: schema,
	}
}

func copyTestTeleportResourceFromTerraform(_ context.Context, object types.Object, resource *testTeleportResource) diag.Diagnostics {
	resource.Name = object.Attrs["name"].(types.String).Value
	resource.Revision = object.Attrs["revision"].(types.String).Value
	resource.Value = object.Attrs["value"].(types.String).Value
	resource.ServerValue = object.Attrs["server_value"].(types.String).Value
	return nil
}

func copyTestTeleportResourceToTerraform(_ context.Context, resource *testTeleportResource, object *types.Object) diag.Diagnostics {
	object.Attrs["id"] = types.String{Value: resource.Name}
	object.Attrs["name"] = types.String{Value: resource.Name}
	object.Attrs["revision"] = types.String{Value: resource.Revision}
	object.Attrs["value"] = types.String{Value: resource.Value}
	object.Attrs["server_value"] = types.String{Value: resource.ServerValue}
	return nil
}

func cloneTestTeleportResource(resource *testTeleportResource) *testTeleportResource {
	clone := *resource
	return &clone
}
