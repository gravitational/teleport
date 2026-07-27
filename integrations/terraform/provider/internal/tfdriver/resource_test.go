// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package tfdriver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/retryutils"
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

func (c *testResourceClient) Get(context.Context, NameIdentifier) (*testTeleportResource, error) {
	if c.upsertCalled {
		clone := *c.after
		return &clone, nil
	}

	clone := *c.before
	return &clone, nil
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

type testCreateResourceClient struct {
	resource *testTeleportResource
}

func (c *testCreateResourceClient) Get(context.Context, NameIdentifier) (*testTeleportResource, error) {
	if c.resource == nil {
		return nil, trace.NotFound("not found")
	}

	clone := *c.resource
	return &clone, nil
}

func (c *testCreateResourceClient) Create(_ context.Context, resource *testTeleportResource) error {
	clone := *resource
	c.resource = &clone
	return nil
}

func (c *testCreateResourceClient) Upsert(context.Context, *testTeleportResource) error {
	return errors.New("upsert not implemented")
}

func (c *testCreateResourceClient) Delete(context.Context, NameIdentifier) error {
	return errors.New("delete not implemented")
}

type testRuntime struct{}

func (t testRuntime) IsConfigured(diag.Diagnostics) bool {
	return true
}

func (t testRuntime) Retry() (retryutils.Retry, error) {
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver: retryutils.NewExponentialDriver(time.Second),
		First:  time.Second,
		Max:    time.Hour,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return retry, nil
}

func (t testRuntime) MaxRetries() int {
	return 10
}

func TestResourceCreateAppliesNormalizerBeforeCreate(t *testing.T) {
	schema := testTeleportResourceSchema()
	client := &testCreateResourceClient{}
	resource := Resource[testTeleportResource, NameIdentifier]{
		resourceClient: client,
		resource: ResourceType[testTeleportResource, NameIdentifier]{
			Kind: "test",
			Codec: ResourceCodecFuncs[testTeleportResource]{
				FromPlanFunc: copyTestTeleportResourceFromTerraform,
				ToStateFunc:  copyTestTeleportResourceToTerraform,
			},
			Normalizer: ResourceNormalizerFuncs[testTeleportResource]{
				Create: func(_ context.Context, resource *testTeleportResource) error {
					resource.Value = "normalized-create-value"
					return nil
				},
			},
			Identifier: NameIdentifierPolicy(path.Root("name"), func(ttr *testTeleportResource) string {
				return ttr.Name
			}),
			ResourceRevision: func(resource *testTeleportResource) string {
				return resource.Revision
			},
		},
		runtime: testRuntime{},
	}

	resp := &tfsdk.CreateResourceResponse{
		State: tfsdk.State{Schema: schema},
	}
	resource.Create(t.Context(), tfsdk.CreateResourceRequest{
		Plan: testTeleportResourcePlan(t, t.Context(), schema, map[string]string{
			"id":           "example",
			"name":         "example",
			"revision":     "1",
			"value":        "planned-value",
			"server_value": "server-value",
		}),
	}, resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	require.Equal(t, "normalized-create-value", client.resource.Value)

	var got types.String
	diags := resp.State.GetAttribute(t.Context(), path.Root("value"), &got)
	assert.False(t, diags.HasError(), diags)
	assert.Equal(t, "normalized-create-value", got.Value)
}

// TestResourceUpdateSetsStateFromRetrieved verifies that Update writes the
// resource read back from Teleport into state after the update lands. This
// guards against regressions where state was populated from the plan-derived
// resource, which drops server-computed values updated during apply.
func TestResourceUpdateSetsStateFromRetrieved(t *testing.T) {
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
			ServerValue: "server-computed-value",
		},
	}

	updateNormalized := false
	resource := Resource[testTeleportResource, NameIdentifier]{
		resourceClient: client,
		resource: ResourceType[testTeleportResource, NameIdentifier]{
			Kind: "test",
			Codec: ResourceCodecFuncs[testTeleportResource]{
				FromPlanFunc: copyTestTeleportResourceFromTerraform,
				ToStateFunc:  copyTestTeleportResourceToTerraform,
			},
			Normalizer: ResourceNormalizerFuncs[testTeleportResource]{
				Update: func(context.Context, *testTeleportResource) error {
					updateNormalized = true
					return nil
				},
			},
			Identifier: NameIdentifierPolicy(path.Root("name"), func(ttr *testTeleportResource) string {
				return ttr.Name
			}),
			ResourceRevision: func(resource *testTeleportResource) string {
				return resource.Revision
			},
		},
		runtime: testRuntime{},
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
	require.True(t, updateNormalized)

	for _, tc := range []struct {
		attr string
		want string
	}{
		{attr: "revision", want: "2"},
		{attr: "value", want: "planned-value"},
		{attr: "server_value", want: "server-computed-value"},
	} {
		var got types.String
		diags := resp.State.GetAttribute(t.Context(), path.Root(tc.attr), &got)
		assert.False(t, diags.HasError(), diags)
		assert.Equal(t, tc.want, got.Value, "state attribute %q should come from the retrieved resource", tc.attr)
	}
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
