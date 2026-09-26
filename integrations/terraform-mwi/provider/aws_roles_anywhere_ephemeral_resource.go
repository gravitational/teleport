/*
Copyright 2015-2026 Gravitational, Inc.

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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gopkg.in/ini.v1"

	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/services/awsra"
)

var _ ephemeral.EphemeralResourceWithConfigure = &AWSRolesAnywhereEphemeralResource{}

func NewAWSRolesAnywhereEphemeralResource() ephemeral.EphemeralResource {
	return &AWSRolesAnywhereEphemeralResource{}
}

type AWSRolesAnywhereEphemeralResource struct {
	pd *providerData // provider data, set in Configure
}

func (r *AWSRolesAnywhereEphemeralResource) Metadata(
	_ context.Context,
	req ephemeral.MetadataRequest,
	resp *ephemeral.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_aws_roles_anywhere"
}

type AWSRolesAnywhereEphemeralResourceModelSelector struct {
	Name types.String `tfsdk:"name"`
}

type AWSRolesAnywhereEphemeralResourceModel struct {
	// Arguments
	Selector        AWSRolesAnywhereEphemeralResourceModelSelector `tfsdk:"selector"`
	RoleARN         types.String                                   `tfsdk:"role_arn"`
	ProfileARN      types.String                                   `tfsdk:"profile_arn"`
	TrustAnchorARN  types.String                                   `tfsdk:"trust_anchor_arn"`
	Region          types.String                                   `tfsdk:"region"`
	SessionDuration timetypes.GoDuration                           `tfsdk:"session_duration"`

	// Attributes
	Output *AWSRolesAnywhereEphemeralResourceModelOutput `tfsdk:"output"`
}

type AWSRolesAnywhereEphemeralResourceModelOutput struct {
	AccessKey types.String `tfsdk:"access_key"`
	SecretKey types.String `tfsdk:"secret_key"`
	Token     types.String `tfsdk:"token"`
}

func (r *AWSRolesAnywhereEphemeralResource) Schema(
	_ context.Context,
	_ ephemeral.SchemaRequest,
	resp *ephemeral.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The AWS Roles Anywhere Ephemeral Resource exchanges a Teleport Workload Identity X.509 SVID for short-lived AWS session credentials that other providers can use. " +
			"A Workload Identity resource must be configured in Teleport to define the SPIFFE identity used in the X.509 SVID. " +
			"The identity that the teleportmwi provider assumes must have permission to use the Workload Identity resource. " +
			"The IAM role, Roles Anywhere profile, and Trust Anchor must already be configured in AWS. " +
			"Credentials cannot be renewed during an operation, so configure a session duration that covers the entire operation. " +
			"AWS profile settings may shorten the requested duration, and the resulting duration must not exceed the IAM role's maximum session duration. ",
		Attributes: map[string]schema.Attribute{
			"selector": schema.SingleNestedAttribute{
				MarkdownDescription: "Selects the Workload Identity to use when issuing the X.509 SVID.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						MarkdownDescription: "The name of the Workload Identity resource.",
						Required:            true,
					},
				},
			},
			"role_arn": schema.StringAttribute{
				MarkdownDescription: "The ARN of the IAM role to assume.",
				Required:            true,
			},
			"profile_arn": schema.StringAttribute{
				MarkdownDescription: "The ARN of the AWS IAM Roles Anywhere profile to use.",
				Required:            true,
			},
			"trust_anchor_arn": schema.StringAttribute{
				MarkdownDescription: "The ARN of the AWS IAM Roles Anywhere trust anchor to use.",
				Required:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "The AWS region of the Roles Anywhere endpoint. Defaults to the region in the trust anchor ARN. This is independent of the region where the AWS provider manages resources.",
				Optional:            true,
			},
			"session_duration": schema.StringAttribute{
				CustomType:          timetypes.GoDurationType{},
				MarkdownDescription: "How long the AWS session credentials should be valid for, using Go duration syntax (for example, `30m` or `1h`). Defaults to 1 hour. Must be between 15 minutes and 12 hours. The AWS profile and IAM role settings also constrain the session duration.",
				Optional:            true,
				Computed:            true,
			},
			// Attributes
			"output": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"access_key": schema.StringAttribute{
						Computed:            true,
						Sensitive:           true,
						MarkdownDescription: "Compatible with the `access_key` argument of the `aws` provider.",
					},
					"secret_key": schema.StringAttribute{
						Computed:            true,
						Sensitive:           true,
						MarkdownDescription: "Compatible with the `secret_key` argument of the `aws` provider.",
					},
					"token": schema.StringAttribute{
						Computed:            true,
						Sensitive:           true,
						MarkdownDescription: "Compatible with the `token` argument of the `aws` provider. Also known as the session token.",
					},
				},
			},
		},
	}
}

func (r *AWSRolesAnywhereEphemeralResource) Configure(
	_ context.Context,
	req ephemeral.ConfigureRequest,
	resp *ephemeral.ConfigureResponse,
) {
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected type", fmt.Sprintf(
			"Expected *providerData, got: %T. Please report this issue to the provider developers.", req.ProviderData,
		))
		return
	}
	r.pd = pd
}

// awsRolesAnywhereConfig builds the service configuration from a model whose
// defaults have already been applied.
func (data AWSRolesAnywhereEphemeralResourceModel) awsRolesAnywhereConfig(dest destination.Destination, sessionDuration time.Duration) (*awsra.Config, error) {
	serviceCfg := &awsra.Config{
		Selector: bot.WorkloadIdentitySelector{
			Name: data.Selector.Name.ValueString(),
		},
		Destination:     dest,
		RoleARN:         data.RoleARN.ValueString(),
		ProfileARN:      data.ProfileARN.ValueString(),
		TrustAnchorARN:  data.TrustAnchorARN.ValueString(),
		Region:          data.Region.ValueString(),
		SessionDuration: sessionDuration,
		// Required by service validation even though OneShot does not renew.
		SessionRenewalInterval: sessionDuration / 2,
	}
	if err := serviceCfg.CheckAndSetDefaults(false); err != nil {
		return nil, err
	}
	return serviceCfg, nil
}

func (r *AWSRolesAnywhereEphemeralResource) Open(
	ctx context.Context,
	req ephemeral.OpenRequest,
	resp *ephemeral.OpenResponse,
) {
	var data AWSRolesAnywhereEphemeralResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.SessionDuration.IsNull() || data.SessionDuration.IsUnknown() {
		data.SessionDuration = timetypes.NewGoDurationValue(time.Hour)
	}

	sessionDuration, diags := data.SessionDuration.ValueGoDuration()
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
	// The value specified can range from 900 seconds (15 minutes) up to the
	// maximum session duration set for the role. The maximum session duration
	// setting can have a value from 1 hour to 12 hours.
	if sessionDuration < awsra.MinAWSSessionDuration || sessionDuration > awsra.MaxAWSSessionDuration {
		resp.Diagnostics.AddAttributeError(
			path.Root("session_duration"),
			"Invalid session duration",
			"Session duration must be between 15 minutes and 12 hours.",
		)
		return
	}

	dest := destination.NewMemory()
	serviceCfg, err := data.awsRolesAnywhereConfig(dest, sessionDuration)
	if err != nil {
		resp.Diagnostics.AddError("Invalid AWS Roles Anywhere configuration", err.Error())
		return
	}

	botCfg := r.pd.newBotConfig()
	botCfg.Services = []bot.ServiceBuilder{awsra.ServiceBuilder(serviceCfg)}
	if err := botCfg.CheckAndSetDefaults(); err != nil {
		resp.Diagnostics.AddError("Error setting defaults for bot config", "Failed to set defaults for bot config: "+err.Error())
		return
	}
	b, err := bot.New(botCfg)
	if err != nil {
		resp.Diagnostics.AddError("Error creating tbot in ephemeral resource", "Failed to create tbot\n"+trace.DebugReport(err))
		return
	}
	if err := b.OneShot(ctx); err != nil {
		resp.Diagnostics.AddError("Error running tbot in ephemeral resource", "Failed to run tbot\n"+trace.DebugReport(err))
		return
	}

	data.Output, err = readAWSRolesAnywhereCredentials(ctx, dest)
	if err != nil {
		resp.Diagnostics.AddError("Error reading AWS credentials", "Failed to read AWS credentials: "+err.Error())
		return
	}
	resp.Diagnostics.Append(resp.Result.Set(ctx, &data)...)
}

func readAWSRolesAnywhereCredentials(ctx context.Context, dest destination.Destination) (*AWSRolesAnywhereEphemeralResourceModelOutput, error) {
	data, err := dest.Read(ctx, "aws_credentials")
	if err != nil {
		return nil, err
	}
	file, err := ini.Load(data)
	if err != nil {
		// INI parsing errors can contain the input, including credentials.
		return nil, errors.New("invalid AWS credentials file")

	}
	profile, err := file.GetSection("default")
	if err != nil {
		return nil, fmt.Errorf("AWS credentials are missing the default profile: %w", err)
	}
	for _, name := range []string{"aws_access_key_id", "aws_secret_access_key", "aws_session_token"} {
		key, err := profile.GetKey(name)
		if err != nil || strings.TrimSpace(key.String()) == "" {
			return nil, fmt.Errorf("AWS credentials are missing %q", name)
		}
	}
	return &AWSRolesAnywhereEphemeralResourceModelOutput{
		AccessKey: types.StringValue(profile.Key("aws_access_key_id").String()),
		SecretKey: types.StringValue(profile.Key("aws_secret_access_key").String()),
		Token:     types.StringValue(profile.Key("aws_session_token").String()),
	}, nil
}
