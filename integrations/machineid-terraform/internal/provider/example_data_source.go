// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	tbotconfig "github.com/gravitational/teleport/lib/tbot/config"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &KubernetesServiceV2DataSource{}

func NewKubernetesV2DataSource() datasource.DataSource {
	return &KubernetesServiceV2DataSource{}
}

// KubernetesServiceV2DataSource defines the data source implementation.
type KubernetesServiceV2DataSource struct {
	bot *botConfig
}

// KubernetesServiceV2DataSourceModel describes the data source data model.
type KubernetesServiceV2DataSourceModel struct {
	Selectors types.List   `tfsdk:"selectors"`
	TTL       types.String `tfsdk:"ttl"`
	Output    types.String `tfsdk:"output"`
}

func (d *KubernetesServiceV2DataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_service_v2"
}

func (d *KubernetesServiceV2DataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Kubernetes Service V2 data source.",

		Attributes: map[string]schema.Attribute{
			"selectors": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Name of the Kubernetes cluster",
						},
						"labels": schema.MapAttribute{
							ElementType:         types.StringType,
							MarkdownDescription: "Labels to select the Kubernetes clusters.",
						},
					},
				},
				MarkdownDescription: "Example configurable attribute",
				Required:            true,
			},
			"ttl": schema.StringAttribute{
				MarkdownDescription: "Example identifier",
				Optional:            true,
				Computed:            true,
			},
			"output": schema.StringAttribute{
				MarkdownDescription: "Example configurable attribute",
				Computed:            true,
			},
		},
	}
}

func (d *KubernetesServiceV2DataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	bot, ok := req.ProviderData.(*botConfig)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *botConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.bot = bot
}

func (d *KubernetesServiceV2DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data KubernetesServiceV2DataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	selectors := make([]*tbotconfig.KubernetesSelector, 0)

	for _, selectorValue := range data.Selectors.Elements() {
		selector, ok := selectorValue.(types.Object)
		if !ok {
			resp.Diagnostics.AddError("Selectors must be an object", fmt.Sprintf("Got %T", selectorValue))
			return
		}

		// TODO: implement label matching
		attrs := selector.Attributes()
		nameValue := attrs["name"]
		name := nameValue.String()
		selectors = append(selectors, &tbotconfig.KubernetesSelector{
			Name: name,
		})
	}

	dest := &tbotconfig.DestinationMemory{}

	output := tbotconfig.KubernetesV2Output{
		Destination:       dest,
		DisableExecPlugin: true,
		Selectors:         selectors,
		CredentialsLifetime: tbotconfig.CredentialsLifetime{
			// TODO: parse TTL
			TTL:             time.Hour,
			RenewalInterval: 30 * time.Minute,
		},
	}

	err := d.bot.runWithServices(ctx, tbotconfig.ServiceConfigs{output})
	if err != nil {
		resp.Diagnostics.AddError("failed to obtain Kubernetes credentials", err.Error())
		return
	}

	// For the purposes of this example code, hardcoding a response value to
	// save into the Terraform state.
	kubeconfig, err := dest.Read(ctx, "kubeconfig.yaml")
	if err != nil {
		resp.Diagnostics.AddError("failed to read kubeconfig", err.Error())
		return
	}

	data.Output = types.StringValue(string(kubeconfig))

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
