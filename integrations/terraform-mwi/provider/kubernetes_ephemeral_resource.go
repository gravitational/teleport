package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ ephemeral.EphemeralResourceWithConfigure = &KubernetesEphemeralResource{}

func NewKubernetesEphemeralResource() *KubernetesEphemeralResource {
	return &KubernetesEphemeralResource{}
}

type KubernetesEphemeralResource struct{}

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
	_, ok := req.ProviderData.(*providerData)
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

	data.ClientKey = types.StringValue("xyzzy") // TODO!

	resp.Diagnostics.Append(resp.Result.Set(ctx, &data)...)
}
