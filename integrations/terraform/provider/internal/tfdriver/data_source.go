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
	"fmt"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdiag"
)

var _ datasource.DataSource = &dataSource[any, Identifier]{}

// DataSourceClient reads a Teleport resource for a data source.
type DataSourceClient[T any, I Identifier] interface {
	// Get reads a Teleport resource by identifier.
	Get(context.Context, I) (*T, error)
}

// DataSourceType describes a Terraform data source.
type DataSourceType[T any, I Identifier] struct {
	NewDataSourceClient func(provider.Provider) DataSourceClient[T, I]
	Kind                string
	Name                string
	Codec               DataSourceCodec[T]
	Identifier          TerraformIdentifierExtractor[I]
}

// Schema returns the data source schema.
func (r DataSourceType[T, I]) Schema(ctx context.Context) (schema.Schema, diag.Diagnostics) {
	return r.Codec.Schema(ctx)
}

// NewDataSource creates the data source.
func (r DataSourceType[T, I]) NewDataSource(_ context.Context, p provider.Provider) (datasource.DataSource, diag.Diagnostics) {
	return dataSource[T, I]{
		dataSourceClient: r.NewDataSourceClient(p),
		dataSource:       r,
	}, nil
}

type dataSource[T any, I Identifier] struct {
	dataSourceClient DataSourceClient[T, I]
	dataSource       DataSourceType[T, I]
}

// Metadata returns the full name of the data source.
func (r dataSource[T, I]) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.dataSource.Name
}

// Schema returns the data source schema.
func (r dataSource[T, I]) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	schema, diags := r.dataSource.Schema(ctx)
	resp.Schema = schema
	resp.Diagnostics = diags
}

// Read reads the Teleport resource.
func (r dataSource[T, I]) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	id, diags := r.dataSource.Identifier(ctx, req.Config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	obj, err := r.dataSourceClient.Get(ctx, id)
	if err != nil {
		resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr(fmt.Sprintf("Error reading %q", r.dataSource.Kind), trace.Wrap(err), r.dataSource.Kind))
		return
	}

	var state types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config T
	diags = r.dataSource.Codec.FromConfig(ctx, state, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Todo: Remove after updating terraform-plugin to >=v1.5.0.
	// terraform-plugin-testing version <1.5.0 requires data resources to
	// implement the 'id' attribute.
	// https://developer.hashicorp.com/terraform/plugin/framework/acctests#no-id-found-in-attributes
	attrs := state.Attributes()
	v, ok := attrs["id"]
	if !ok || v.IsNull() {
		attrs["id"] = types.StringValue(id.String())
		state, diags = types.ObjectValue(state.AttributeTypes(ctx), attrs)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	state, diags = r.dataSource.Codec.ToState(ctx, obj, &state)
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
