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

package v1

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	clientiprestrictionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/clientiprestriction/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
)

// TestGenSchemaClientIPRestriction verifies the generated schema is well-formed
// and that ClientIPRestriction's server-managed singleton fields are computed
// while the user-configurable spec stays settable.
func TestGenSchemaClientIPRestriction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	schema, diags := GenSchemaClientIPRestriction(ctx)
	require.False(t, diags.HasError(), "unexpected diagnostics: %v", diags)

	// The name is fixed and the rest of the metadata is set by the server, so
	// these must be computed (the user never provides them).
	for _, name := range []string{"kind", "sub_kind", "version", "metadata"} {
		attribute, ok := schema.Attributes[name]
		require.True(t, ok, "missing attribute %q", name)
		require.True(t, attribute.Computed, "attribute %q should be computed", name)
	}

	// status is server-managed enforcement state; per RFD 153 the provider must
	// not track it, so it is excluded from the schema entirely.
	_, ok := schema.Attributes["status"]
	require.False(t, ok, "status should be excluded from the schema")

	// spec holds allowed_cidrs and is the only user-configurable part.
	spec, ok := schema.Attributes["spec"]
	require.True(t, ok, "missing spec attribute")
	require.True(t, spec.Optional, "spec should be optional")
}

// TestCopyClientIPRestrictionRoundTrip ensures a ClientIPRestriction survives a
// proto -> Terraform -> proto round trip through the generated copy functions,
// covering the user spec and the computed metadata. The server-managed status
// is excluded from the schema, so it must not survive the round trip.
func TestCopyClientIPRestrictionRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	schema, diags := GenSchemaClientIPRestriction(ctx)
	require.False(t, diags.HasError(), "unexpected diagnostics: %v", diags)
	objectType, ok := schema.AttributeType().(types.ObjectType)
	require.True(t, ok, "schema attribute type should be an object")

	want := &clientiprestrictionv1.ClientIPRestriction{
		Kind:    "client_ip_restriction",
		Version: "v1",
		Metadata: &headerv1.Metadata{
			Name: "client-ip-restriction",
		},
		Spec: &clientiprestrictionv1.ClientIPRestrictionSpec{
			AllowedCidrs: []string{"10.0.0.0/8", "192.168.0.0/16"},
		},
		Status: &clientiprestrictionv1.ClientIPRestrictionStatus{
			State: "active",
		},
	}

	tf := types.Object{AttrTypes: objectType.AttrTypes, Attrs: make(map[string]attr.Value)}
	diags = CopyClientIPRestrictionToTerraform(ctx, want, &tf)
	require.False(t, diags.HasError(), "copy to terraform: %v", diags)

	got := &clientiprestrictionv1.ClientIPRestriction{}
	diags = CopyClientIPRestrictionFromTerraform(ctx, tf, got)
	require.False(t, diags.HasError(), "copy from terraform: %v", diags)

	require.Equal(t, want.GetKind(), got.GetKind())
	require.Equal(t, want.GetVersion(), got.GetVersion())
	require.Equal(t, want.GetMetadata().GetName(), got.GetMetadata().GetName())
	require.Equal(t, want.GetSpec().GetAllowedCidrs(), got.GetSpec().GetAllowedCidrs())
	// status is excluded from the schema, so it is dropped on the round trip.
	require.Empty(t, got.GetStatus().GetState())
}
