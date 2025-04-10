// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot"
	"github.com/gravitational/trace"
	"log/slog"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	tbotconfig "github.com/gravitational/teleport/lib/tbot/config"
)

// Ensure ScaffoldingProvider satisfies various provider interfaces.
var _ provider.Provider = &ScaffoldingProvider{}
var _ provider.ProviderWithFunctions = &ScaffoldingProvider{}
var _ provider.ProviderWithEphemeralResources = &ScaffoldingProvider{}

// ScaffoldingProvider defines the provider implementation.
type ScaffoldingProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// ScaffoldingProviderModel describes the provider data model.
type ScaffoldingProviderModel struct {
	// Addr Teleport address
	Addr types.String `tfsdk:"addr"`
	// JoinMethod is the MachineID join method.
	JoinMethod types.String `tfsdk:"join_method"`
	// JoinMethod is the MachineID join token.
	JoinToken types.String `tfsdk:"join_token"`
	// AudienceTag is the audience  tag for the `terraform` join method
	AudienceTag types.String `tfsdk:"audience_tag"`
}

func (p *ScaffoldingProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "machineid"
	resp.Version = p.version
}

func (p *ScaffoldingProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"addr": schema.StringAttribute{
				MarkdownDescription: "host:port of the Teleport address. This can be the Teleport Proxy Service address (port 443 or 4080) or the Teleport Auth Service address (port 3025).",
				Required:            true,
			},
			"join_method": schema.StringAttribute{
				MarkdownDescription: "See [the join method reference](../join-methods.mdx) for possible values. You must use [a delegated join method](../join-methods.mdx#secret-vs-delegated).",
				Required:            true,
			},
			"join_token": schema.StringAttribute{
				MarkdownDescription: "Name of the token used for MachineID joining.",
				Required:            true,
			},
			"audience_tag": schema.StringAttribute{
				MarkdownDescription: "Name of the optional audience tag used for native Machine ID joining with the `terraform` method.",
				Optional:            true,
			},
		},
	}
}

func (p *ScaffoldingProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data ScaffoldingProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	bot := &botConfig{
		addr:        data.Addr.ValueString(),
		joinMethod:  data.JoinMethod.ValueString(),
		joinToken:   data.JoinToken.ValueString(),
		audienceTag: data.AudienceTag.ValueString(),
		lock:        sync.Mutex{},
		storage:     &tbotconfig.StorageConfig{Destination: &tbotconfig.DestinationMemory{}},
	}
	// Configuration values are now available.
	// if data.Endpoint.IsNull() { /* ... */ }

	_, err := bot.preflight(ctx)
	if err != nil {
		resp.Diagnostics.AddError("error during provider preflight checks", err.Error())
		return
	}

	resp.DataSourceData = bot

}

func (p *ScaffoldingProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

func (p *ScaffoldingProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{}
}

func (p *ScaffoldingProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewKubernetesV2DataSource,
	}
}

func (p *ScaffoldingProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ScaffoldingProvider{
			version: version,
		}
	}
}

type botConfig struct {
	addr        string
	joinMethod  string
	joinToken   string
	audienceTag string

	lock    sync.Mutex
	storage *tbotconfig.StorageConfig
}

func (c *botConfig) preflight(ctx context.Context) (*proto.PingResponse, error) {
	credential := &tbotconfig.UnstableClientCredentialOutput{}
	services := tbotconfig.ServiceConfigs{credential}

	err := c.runWithServices(ctx, services)
	if err != nil {
		return nil, trace.Wrap(err, "tbot preflight run")
	}

	clt, err := client.New(ctx, client.Config{
		Addrs:       []string{c.addr},
		Credentials: []client.Credentials{credential},
	})
	if err != nil {
		return nil, trace.Wrap(err, "building test client")
	}

	pong, err := clt.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "pinging Teleport cluster")
	}

	return &pong, nil
}

func (c *botConfig) runWithServices(ctx context.Context, services tbotconfig.ServiceConfigs) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	cfg := &tbotconfig.BotConfig{
		AuthServer: c.addr,
		Oneshot:    true,
		Onboarding: tbotconfig.OnboardingConfig{
			TokenValue: c.joinToken,
			JoinMethod: apitypes.JoinMethod(c.joinMethod),
			Terraform: tbotconfig.TerraformOnboardingConfig{
				AudienceTag: c.audienceTag,
			},
		},
		CredentialLifetime: tbotconfig.CredentialLifetime{
			TTL:             time.Hour,
			RenewalInterval: 20 * time.Minute,
		},
		Storage:  c.storage,
		Services: services,
	}

	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err, "checking bot config")
	}

	bot := tbot.New(cfg, slog.Default())
	return trace.Wrap(bot.Run(ctx))
}
