package provider

import (
	"context"
	"log/slog"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot"
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
	ProxyServer types.String `tfsdk:"proxy_server"`
	JoinMethod  types.String `tfsdk:"join_method"`
	JoinToken   types.String `tfsdk:"join_token"`
}

type providerData struct {
	bot          *tbot.Bot
	newBotConfig func() *config.BotConfig
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

	// TODO: Can multiple ephemeral resources/data sources initialize at the
	// same time? If so, we'll need some kind of protection over this.
	botInternalStore := config.DestinationMemory{}
	if err := botInternalStore.CheckAndSetDefaults(); err != nil {
		resp.Diagnostics.AddError(
			"Error setting defaults for bot internal store",
			"Failed to set defaults for bot internal store: "+err.Error(),
		)
		return
	}

	newBotConfig := func() *config.BotConfig {
		return &config.BotConfig{
			Version:     "v2",
			ProxyServer: data.ProxyServer.ValueString(),
			Storage: &config.StorageConfig{
				Destination: &botInternalStore,
			},
			Onboarding: config.OnboardingConfig{
				JoinMethod: apitypes.JoinMethod(data.JoinMethod.ValueString()),
				TokenValue: data.JoinToken.ValueString(),
			},
			Oneshot: true,
		}
	}

	botCfg := newBotConfig()
	if err := botCfg.CheckAndSetDefaults(); err != nil {
		resp.Diagnostics.AddError(
			"Error setting defaults for bot config",
			"Failed to set defaults for bot config: "+err.Error(),
		)
		return
	}
	bot := tbot.New(botCfg, slog.Default())

	// Run bot just to validate that the configuration is correct.
	if err := bot.Run(ctx); err != nil {
		resp.Diagnostics.AddError(
			"Error running tbot in provider",
			"Failed to run tbot: "+err.Error(),
		)
		return
	}

	providerData := providerData{
		bot:          bot,
		newBotConfig: newBotConfig,
	}
	resp.DataSourceData = &providerData
	resp.EphemeralResourceData = &providerData
}

func (p *Provider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		NewKubernetesEphemeralResource,
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
