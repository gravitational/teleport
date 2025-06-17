/*
Copyright 2015-2025 Gravitational, Inc.

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

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/config"
)

var (
	_ provider.Provider                       = &Provider{}
	_ provider.ProviderWithEphemeralResources = &Provider{}
)

func New() func() provider.Provider {
	return func() provider.Provider {
		return &Provider{}
	}
}

type Provider struct {
}

type ProviderModel struct {
	// ProxyServer is the address of the Teleport proxy server. This should
	// exclude the scheme, but include the port.
	ProxyServer types.String `tfsdk:"proxy_server"`
	// JoinMethod is the method used to join the cluster.
	// Must be specified.
	JoinMethod types.String `tfsdk:"join_method"`
	// JoinToken is the token used to join the cluster.
	// Must be specified.
	JoinToken types.String `tfsdk:"join_token"`
}

func (p *Provider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "teleportmwi"
}

func (p *Provider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"proxy_server": schema.StringAttribute{
				MarkdownDescription: "The address of the Teleport Proxy service. This should exclude the scheme but should include the port.",
				Required:            true,
			},
			"join_method": schema.StringAttribute{
				MarkdownDescription: "The join method to use to authenticate to the Teleport cluster.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(config.SupportedJoinMethods...),
					// Explicitly prohibit the use of the token join method
					// as we won't be able to persist state for it to work
					// effectively.
					stringvalidator.NoneOf(string(apitypes.JoinMethodToken)),
				},
			},
			"join_token": schema.StringAttribute{
				MarkdownDescription: "The name of the join token to use to authenticate to the Teleport cluster.",
				Required:            true,
			},
		},
	}
}

func (p *Provider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data ProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (p *Provider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		NewKubernetesEphemeralResource,
	}
}

func (p *Provider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewKubernetesDataSource,
	}
}

func (p *Provider) Resources(ctx context.Context) []func() resource.Resource {
	// We have to implement this method to satisfy the provider.Provider
	// interface - but we don't have any resources to return.
	return []func() resource.Resource{}
}
