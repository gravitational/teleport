package provider

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/gravitational/teleport/lib/tbot"
	"github.com/gravitational/teleport/lib/tbot/config"
)

var _ ephemeral.EphemeralResourceWithConfigure = &KubernetesEphemeralResource{}

func NewKubernetesEphemeralResource() ephemeral.EphemeralResource {
	return &KubernetesEphemeralResource{}
}

type KubernetesEphemeralResource struct {
	pd *providerData
}

func (r *KubernetesEphemeralResource) Metadata(
	_ context.Context,
	req ephemeral.MetadataRequest,
	resp *ephemeral.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes"
}

type KubernetesEphemeralResourceModelSelector struct {
	Name types.String `tfsdk:"name"`
}

type KubernetesEphemeralResourceModel struct {
	// Input

	Selector      KubernetesEphemeralResourceModelSelector `tfsdk:"selector"`
	CredentialTTL timetypes.GoDuration                     `tfsdk:"credential_ttl"`

	// Output
	ClientKey            types.String `tfsdk:"client_key"`
	Host                 types.String `tfsdk:"host"`
	TLSServerName        types.String `tfsdk:"tls_server_name"`
	ClientCertificate    types.String `tfsdk:"client_certificate"`
	ClusterCACertificate types.String `tfsdk:"cluster_ca_certificate"`
}

func (r *KubernetesEphemeralResource) Schema(
	_ context.Context,
	_ ephemeral.SchemaRequest,
	resp *ephemeral.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		// TODO
		MarkdownDescription: "TODO",

		Attributes: map[string]schema.Attribute{
			// Input
			"selector": schema.SingleNestedAttribute{
				MarkdownDescription: "TODO",
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						MarkdownDescription: "TODO",
						Required:            true,
					},
				},
				Required: true,
			},
			"credential_ttl": schema.StringAttribute{
				CustomType:          timetypes.GoDurationType{},
				MarkdownDescription: "TODO",
				Required:            true,
			},
			// Output
			"client_key": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "TODO",
			},
			"host": schema.StringAttribute{
				Computed: true,
			},
			"tls_server_name": schema.StringAttribute{
				Computed: true,
			},
			"client_certificate": schema.StringAttribute{
				Computed: true,
			},
			"cluster_ca_certificate": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (d *KubernetesEphemeralResource) Configure(
	ctx context.Context,
	req ephemeral.ConfigureRequest,
	resp *ephemeral.ConfigureResponse,
) {
	// TODO: wrap in helper?
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected type",
			fmt.Sprintf(
				"Expected *providerData, got: %T. Please report this issue to the provider developers.",
				req.ProviderData,
			),
		)

		return
	}
	// TODO: end wrap in helper?
	d.pd = pd
}

func (r *KubernetesEphemeralResource) Open(
	ctx context.Context,
	req ephemeral.OpenRequest,
	resp *ephemeral.OpenResponse,
) {
	var data KubernetesEphemeralResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dest := &config.DestinationMemory{}
	if err := dest.CheckAndSetDefaults(); err != nil {
		panic("boo")
		return
	}
	botCfg := r.pd.newBotConfig()
	botCfg.Services = config.ServiceConfigs{
		&config.KubernetesV2Output{
			Destination: dest,
			Selectors: []*config.KubernetesSelector{
				{
					Name: data.Selector.Name.String(),
				},
			},
			DisableExecPlugin: true,
		},
	}
	if err := botCfg.CheckAndSetDefaults(); err != nil {
		resp.Diagnostics.AddError(
			"Error setting defaults for bot config",
			"Failed to set defaults for bot config: "+err.Error(),
		)
		return
	}
	bot := tbot.New(botCfg, slog.Default())
	if err := bot.Run(ctx); err != nil {
		resp.Diagnostics.AddError(
			"Error running tbot in resource",
			"Failed to run tbot: "+err.Error(),
		)
		return
	}

	// TODO: parse kubeconfig out of destination...
	destData, err := dest.Read(ctx, "kubeconfig.yaml")
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading kubeconfig",
			"Failed to read kubeconfig: "+err.Error(),
		)
		return
	}

	cfg, err := clientcmd.Load(destData)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing kubeconfig",
			"Failed to load kubeconfig: "+err.Error(),
		)
		return
	}

	// TODO: make this bit nil-safe
	kubectx := cfg.Contexts[cfg.CurrentContext]
	cluster := cfg.Clusters[kubectx.Cluster]
	user := cfg.AuthInfos[kubectx.AuthInfo]

	// TODO: Probably fix this...
	data.Host = types.StringValue(cluster.Server)
	data.TLSServerName = types.StringValue(cluster.TLSServerName)
	data.ClientKey = types.StringValue(string(user.ClientKeyData))
	data.ClientCertificate = types.StringValue(string(user.ClientCertificateData))
	data.ClusterCACertificate = types.StringValue(string(cluster.CertificateAuthorityData))

	resp.Diagnostics.Append(resp.Result.Set(ctx, &data)...)
}
