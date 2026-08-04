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
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func TestResourceCodecFuncsUsesSourceSpecificDecode(t *testing.T) {
	codec := ResourceCodecFuncs[string]{
		SchemaFunc: func(context.Context) (tfsdk.Schema, diag.Diagnostics) {
			return tfsdk.Schema{}, nil
		},
		FromPlanFunc: func(_ context.Context, _ types.Object, out *string) diag.Diagnostics {
			*out = "plan"
			return nil
		},
		FromStateFunc: func(_ context.Context, _ types.Object, out *string) diag.Diagnostics {
			*out = "state"
			return nil
		},
		ToStateFunc: func(_ context.Context, in *string, _ *types.Object) diag.Diagnostics {
			*in += ":to-state"
			return nil
		},
	}

	var fromPlan string
	diags := codec.FromPlan(t.Context(), types.Object{}, &fromPlan)
	require.False(t, diags.HasError(), diags)
	require.Equal(t, "plan", fromPlan)

	var fromState string
	diags = codec.FromState(t.Context(), types.Object{}, &fromState)
	require.False(t, diags.HasError(), diags)
	require.Equal(t, "state", fromState)

	diags = codec.ToState(t.Context(), &fromState, &types.Object{})
	require.False(t, diags.HasError(), diags)
	require.Equal(t, "state:to-state", fromState)
}

func TestDataSourceCodecFuncsUsesConfigDecode(t *testing.T) {
	codec := DataSourceCodecFuncs[string]{
		SchemaFunc: func(context.Context) (tfsdk.Schema, diag.Diagnostics) {
			return tfsdk.Schema{}, nil
		},
		FromConfigFunc: func(_ context.Context, _ types.Object, out *string) diag.Diagnostics {
			*out = "config"
			return nil
		},
		ToStateFunc: func(_ context.Context, in *string, _ *types.Object) diag.Diagnostics {
			*in += ":to-state"
			return nil
		},
	}

	var fromConfig string
	diags := codec.FromConfig(t.Context(), types.Object{}, &fromConfig)
	require.False(t, diags.HasError(), diags)
	require.Equal(t, "config", fromConfig)

	diags = codec.ToState(t.Context(), &fromConfig, &types.Object{})
	require.False(t, diags.HasError(), diags)
	require.Equal(t, "config:to-state", fromConfig)
}
