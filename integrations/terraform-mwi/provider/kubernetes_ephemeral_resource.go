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
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ ephemeral.EphemeralResourceWithConfigure = &KubernetesEphemeralResource{}

func NewKubernetesEphemeralResource() ephemeral.EphemeralResource {
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
	// Arguments
	Selector      KubernetesEphemeralResourceModelSelector `tfsdk:"selector"`
	CredentialTTL timetypes.GoDuration                     `tfsdk:"credential_ttl"`

	// Attributes
	Output *KubernetesEphemeralResourceModelOutput `tfsdk:"output"`
}

type KubernetesEphemeralResourceModelOutput struct {
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
		MarkdownDescription: "The Kubernetes Ephemeral Resource provides credentials to allow other providers to access Kubernetes cluster through Teleport Machine & Workload Identity.",

		Attributes: map[string]schema.Attribute{
			// Arguments
			"selector": schema.SingleNestedAttribute{
				MarkdownDescription: "Selects the Kubernetes cluster to connect to.",
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						MarkdownDescription: "The name of the Kubernetes cluster to connect to.",
						Required:            true,
					},
				},
				Required: true,
			},
			"credential_ttl": schema.StringAttribute{
				CustomType:          timetypes.GoDurationType{},
				MarkdownDescription: "How long the issued credentials should be valid for. Defaults to 30 minutes.",
				Optional:            true,
				Computed:            true,
			},
			// Attributes
			"output": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"client_key": schema.StringAttribute{
						Computed:            true,
						Sensitive:           true,
						MarkdownDescription: "Compatible with the `client_key` argument of the `kubernetes` provider.",
					},
					"host": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Compatible with the `host` argument of the `kubernetes` provider.",
					},
					"tls_server_name": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Compatible with the `tls_server_name` argument of the `kubernetes` provider.",
					},
					"client_certificate": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Compatible with the `client_certificate` argument of the `kubernetes` provider.",
					},
					"cluster_ca_certificate": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Compatible with the `cluster_ca_certificate` argument of the `kubernetes` provider.",
					},
				},
			},
		},
	}
}

func (d *KubernetesEphemeralResource) Configure(
	ctx context.Context,
	req ephemeral.ConfigureRequest,
	resp *ephemeral.ConfigureResponse,
) {
	// TODO: Fetch bot config data from the provider.
}

func (r *KubernetesEphemeralResource) loadModelAndSetDefaults(
	ctx context.Context,
	req ephemeral.OpenRequest,
	resp *ephemeral.OpenResponse,
) KubernetesEphemeralResourceModel {
	var data KubernetesEphemeralResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return data
	}

	// Set default for credential TTL if not provided.
	if data.CredentialTTL.IsNull() || data.CredentialTTL.IsUnknown() {
		data.CredentialTTL = timetypes.NewGoDurationValue(
			time.Minute * 30,
		)
	}

	return data
}

func (r *KubernetesEphemeralResource) Open(
	ctx context.Context,
	req ephemeral.OpenRequest,
	resp *ephemeral.OpenResponse,
) {
	data := r.loadModelAndSetDefaults(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		return
	}

	out := KubernetesEphemeralResourceModelOutput{
		Host: types.StringValue(
			fmt.Sprintf("Hello, %s!", data.Selector.Name.ValueString()),
		),
	}

	data.Output = &out
	resp.Diagnostics.Append(resp.Result.Set(ctx, &data)...)
}
