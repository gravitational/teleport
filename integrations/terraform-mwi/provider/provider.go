package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ provider.Provider                       = &Provider{}
	_ provider.ProviderWithEphemeralResources = &Provider{}
)

type Provider struct {
}

type ProviderModel struct {
	ProxyServer types.String `tfsdk:"proxy_server"`
	JoinMethod  types.String `tfsdk:"join_method"`
	JoinToken   types.String `tfsdk:"join_token"`
}

type providerData struct {
}

func (p *Provider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "teleportmwi"
}

func (p *Provider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			// TODO: Revisit descriptions etc.
			"proxy_server": schema.StringAttribute{
				MarkdownDescription: "The address of your Teleport Proxy",
				Required:            true,
			},
			"join_method": schema.StringAttribute{
				MarkdownDescription: "The method used to join the cluster",
				Required:            true,
			},
			"join_token": schema.StringAttribute{
				MarkdownDescription: "The token used to join the cluster",
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

	providerData := providerData{}
	resp.DataSourceData = &providerData
	resp.EphemeralResourceData = &providerData
}

func (p *Provider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		// TODO
	}
}

func (p *Provider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		// TODO
	}
}

func (p *Provider) Resources(ctx context.Context) []func() resource.Resource {
	// We have to implement this method to satisfy the provider.Provider
	// interface - but we don't have any resources to return.
	return []func() resource.Resource{}
}
