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

package provider

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/scopes"
)

// dataSourceTeleportType is the resource
type dataSourceTeleportType[T any, I fmt.Stringer] struct {
	newDataSourceClient func(*client.Client) dataSourceClient[T, I]
	RetryConfig         RetryConfig
	kind                string
	schema              func(context.Context) (tfsdk.Schema, diag.Diagnostics)
	toTerraform         func(context.Context, *T, *types.Object) diag.Diagnostics
	identifier          func(context.Context, tfsdk.Config) (I, diag.Diagnostics)
}

type NameIdentifier struct {
	Name string
}

func (n NameIdentifier) String() string {
	return n.Name
}

type ScopeQualifiedNameIdentifier struct {
	SQN scopes.QualifiedName
}

func (n ScopeQualifiedNameIdentifier) String() string {
	return n.SQN.String()
}

type CompositeIdentifier struct {
	Prefix string
	Name   string
}

func (n CompositeIdentifier) String() string {
	return formatID(n.Prefix, n.Name)
}

type GetResourceRequest[I any] struct {
	WithSecrets bool
	Identifier  I
}

type dataSourceClient[T, I any] interface {
	Get(context.Context, GetResourceRequest[I]) (*T, error)
}

// GetSchema returns the data source schema
func (r dataSourceTeleportType[T, I]) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return r.schema(ctx)
}

// NewDataSource creates the empty data source
func (r dataSourceTeleportType[T, I]) NewDataSource(_ context.Context, p tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	return dataSourceTeleport[T, I]{
		dataSourceClient: r.newDataSourceClient(p.(*Provider).Client),
		dataSource:       r,
	}, nil
}

// dataSourceTeleport is the resource
type dataSourceTeleport[T any, I fmt.Stringer] struct {
	dataSourceClient[T, I]
	dataSource dataSourceTeleportType[T, I]
}

// Read retrieves a Teleport resource.
func (r dataSourceTeleport[T, I]) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	id, diags := r.dataSource.identifier(ctx, req.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	obj, err := r.dataSourceClient.Get(ctx, GetResourceRequest[I]{Identifier: id, WithSecrets: true})
	if err != nil {
		resp.Diagnostics.Append(diagFromWrappedErr(fmt.Sprintf("Error reading %s", r.dataSource.kind), trace.Wrap(err), r.dataSource.kind))
		return
	}

	var state types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Todo: Remove after updating terraform-plugin to >=v1.5.0.
	// terraform-plugin-testing version <1.5.0 requires data resources to
	// implement the 'id' attribute.
	// https://developer.hashicorp.com/terraform/plugin/framework/acctests#no-id-found-in-attributes
	v, ok := state.Attrs["id"]
	if !ok || v.IsNull() {
		state.Attrs["id"] = types.String{Value: id.String()}
	}

	diags = r.dataSource.toTerraform(ctx, obj, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
