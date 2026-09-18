/*
Copyright 2022 Gravitational, Inc.

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

package legacy

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/scopes"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/slices"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdiag"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// GenSchemaBot returns the schema of the `teleport_bot` resource.
//
// This is quite different to our other Terraform resources because it was hand-
// written rather than generated from protobufs. As such, it originally did not
// follow the RFD 153 conventions of having `metadata` and `spec` attributes,
// but we added them later.
//
// In order to make migration as seamless as possible, and to avoid breaking the
// user's existing configuration or introducing a new `teleport_bot_v2` resource,
// this resource supports both the old (top-level) and new (spec.* and metadata.*)
// attributes.
//
// See resourceTeleportBot.botFromProto for more information.
func GenSchemaBot(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Computed:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
				Type:          types.StringType,
			},
			"kind": {
				Computed:      true,
				Description:   "The kind of resource represented.",
				Optional:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
				Type:          types.StringType,
			},
			"sub_kind": {
				Computed:      true,
				Description:   "Differentiates variations of the same kind. All resources should contain one, even if it is never populated.",
				Optional:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
				Type:          types.StringType,
			},
			"version": {
				Computed:      true,
				Description:   "The version of the resource being represented.",
				Optional:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
				Type:          types.StringType,
				Validators:    []tfsdk.AttributeValidator{tfschema.UseVersionBetween(1, 1)},
			},
			"metadata": {
				Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
					"description": {
						Description: "Description is object description",
						Optional:    true,
						Type:        types.StringType,
					},
					"expires": {
						Description: "Expires is a global expiry time header can be set on any resource in the system.",
						Optional:    true,
						Type:        tfschema.UseRFC3339Time(),
						Validators:  []tfsdk.AttributeValidator{tfschema.MustTimeBeInFuture()},
					},
					"labels": {
						Description: "Labels is a set of labels",
						Optional:    true,
						Type:        types.MapType{ElemType: types.StringType},
						Validators:  []tfsdk.AttributeValidator{tfschema.UseMapKeysPresentValidator("teleport.dev/origin")},
					},
					"name": {
						Description:   "Name is an object name",
						PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
						Required:      true,
						Type:          types.StringType,
					},
					"namespace": {
						Computed:      true,
						Description:   "Namespace is object namespace. The field should be called \"namespace\" when it returns in Teleport 2.4.",
						Optional:      true,
						PlanModifiers: []tfsdk.AttributePlanModifier{resource.UseStateForUnknown()},
						Type:          types.StringType,
					},
					"revision": {
						Description: "Revision is an opaque identifier which tracks the versions of a resource over time. Clients should ignore and not alter its value but must return the revision in any updates of a resource.",
						Optional:    true,
						Type:        types.StringType,
					},
				}),
				Description: "Common metadata that all resources share",
				Optional:    true,
				Validators: []tfsdk.AttributeValidator{
					requiredUnlessLegacyValidator{},
				},
			},
			"spec": {
				Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
					"traits": tfschema.GenSchemaTraits(ctx, tfsdk.Attribute{
						Description: "The traits that will be associated with the bot for the purposes of role templating.\n\nWhere multiple specified with the same name, these will be merged by the server.",
					}),
					"roles": {
						Optional:    true,
						Description: "A list of roles the created bot should be allowed to assume via role impersonation.",
						Type:        types.ListType{ElemType: types.StringType},
					},
					"max_session_ttl": {
						Optional:    true,
						Computed:    true,
						Description: "The max session TTL value for the bot's internal role. Unless specified, bots may not request a value beyond the default maximum TTL of 12 hours. This value may not be larger than 7 days (168 hours).",
						Type:        tfschema.DurationType{},
					},
				}),
				Description: "The configured properties of a bot.",
				Optional:    true,
				Validators: []tfsdk.AttributeValidator{
					requiredUnlessLegacyValidator{},
				},
			},
			"status": {
				Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
					"user_name": {
						Description: "The name of the user associated with the bot.",
						Type:        types.StringType,
						Computed:    true,
					},
					"role_name": {
						Description: "The name of the role associated with the bot.",
						Type:        types.StringType,
						Computed:    true,
					},
				}),
				Description: "Fields that are set by the server as results of operations. These should not be modified by users.",
				Computed:    true,
			},
			"scope": {
				Description:   "Scope is the scope of the bot resource. Leave empty for unscoped bots.",
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
				Optional:      true,
				Type:          types.StringType,
			},

			// Deprecated fields.
			"name": {
				Type:               types.StringType,
				Optional:           true,
				Description:        "The name of the bot, i.e. the unprefixed User name",
				DeprecationMessage: "Deprecated. Used `metadata.name` instead.",
				Validators: []tfsdk.AttributeValidator{
					rfd153OnlyValidator{},
				},
				PlanModifiers: []tfsdk.AttributePlanModifier{resource.RequiresReplace()},
			},
			"user_name": {
				Type:               types.StringType,
				Computed:           true,
				Description:        "The name of the generated bot user",
				DeprecationMessage: "Deprecated. Use `status.user_name` instead.",
			},
			"role_name": {
				Type:               types.StringType,
				Computed:           true,
				Description:        "The name of the generated bot role",
				DeprecationMessage: "Deprecated. Use `status.role_name` instead.",
			},
			"token_ttl": {
				Type:               types.StringType,
				Optional:           true,
				Computed:           true,
				DeprecationMessage: "Deprecated. This field is not required anymore and has no effect.",
			},
			"token_id": {
				Type: types.StringType,
				// Implementation note: this is not used anymore, we can skip this
				// This will go away eventually when we'll generate the bot provider instead
				Optional:           true,
				Sensitive:          true,
				DeprecationMessage: "Deprecated. This field is not required anymore and has no effect.",
			},
			"roles": {
				Type: types.ListType{
					ElemType: types.StringType,
				},
				Optional:           true,
				Description:        "A list of roles the created bot should be allowed to assume via role impersonation.",
				DeprecationMessage: "Deprecated. Use `spec.roles` instead.",
				Validators: []tfsdk.AttributeValidator{
					rfd153OnlyValidator{},
				},
			},
			"traits": tfschema.GenSchemaTraits(ctx, tfsdk.Attribute{
				DeprecationMessage: "Deprecated. Use `spec.traits` instead.",
				Validators: []tfsdk.AttributeValidator{
					rfd153OnlyValidator{},
				},
			}),
		},
	}, nil
}

var _ resource.Resource = &resourceTeleportBot{}

// resourceTeleportBotType is the resource metadata type
type resourceTeleportBotType struct{}

// NewResource creates the empty resource
func (r resourceTeleportBotType) NewResource(_ context.Context, p provider.Provider) (resource.Resource, diag.Diagnostics) {
	return resourceTeleportBot{
		p: p.(Provider),
	}, nil
}

// resourceTeleportBot is the resource
type resourceTeleportBot struct {
	p Provider
}

// Metadata returns the full name of the resource
func (r resourceTeleportBot) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "teleport_bot"
}

// GetSchema returns the resource schema
func (r resourceTeleportBot) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	// It's unusual for this provider, but we'll hand-write the schema here as
	// bots do not have any server-side resources of their own.
	return GenSchemaBot(ctx)
}

func (r resourceTeleportBot) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.p.IsConfigured(resp.Diagnostics) {
		return
	}

	var plan Bot
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	response, err := r.p.Client().BotServiceClient().
		CreateBot(ctx, &machineidv1.CreateBotRequest{Bot: plan.ToProto()})
	if err != nil {
		resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr("Error creating Bot", trace.Wrap(err), "bot"))
		return
	}

	plan, diags = r.botFromProto(ctx, response, plan, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceTeleportBot) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Bot
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	bot, err := r.p.Client().BotServiceClient().GetBot(ctx, &machineidv1.GetBotRequest{
		BotName: state.GetName(),
		Scope:   state.GetScope(),
	})
	switch {
	case trace.IsNotFound(err):
		resp.State.RemoveResource(ctx)
		return
	case err != nil:
		resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr("Error reading Bot", trace.Wrap(err), "bot"))
		return
	}

	state, diags = r.botFromProto(ctx, bot, state, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// botFromProto converts the server/protobuf representation of a bot to Terraform
// using the given "base" (plan or state) to determine whether to fill the legacy
// or new attributes.
//
// NOTE: Terraform is *very* fussy about non-computed attributes changing between
// plan and apply, even in benign ways such as null becoming an empty list, so
// exercise caution!
func (r resourceTeleportBot) botFromProto(ctx context.Context, bot *machineidv1.Bot, base Bot, importState bool) (Bot, diag.Diagnostics) {
	schema, _ := GenSchemaBot(ctx)

	attrTypes := func(key string) map[string]attr.Type {
		result := make(map[string]attr.Type)
		for k, v := range schema.Attributes[key].Attributes.GetAttributes() {
			result[k] = v.GetType()
		}
		return result
	}

	stringValue := func(v string) types.String {
		if v == "" {
			return types.StringNull()
		}
		return types.StringValue(v)
	}

	timeValue := func(v *timestamppb.Timestamp) tfschema.TimeValue {
		if v == nil {
			return tfschema.NullTime()
		}
		return tfschema.NewTime(v.AsTime())
	}

	durationValue := func(v *durationpb.Duration) tfschema.DurationValue {
		if v == nil {
			return tfschema.DurationValue{Null: true}
		}
		return tfschema.DurationValue{Value: v.AsDuration()}
	}

	attrNull := func(obj types.Object, name string) bool {
		if obj.IsNull() {
			return true
		}
		if obj.Attributes() == nil {
			return true
		}
		attr, ok := obj.Attributes()[name]
		return !ok || attr.IsNull()
	}

	statusValue, diags := types.ObjectValue(attrTypes("status"), map[string]attr.Value{
		"user_name": stringValue(bot.GetStatus().GetUserName()),
		"role_name": stringValue(bot.GetStatus().GetRoleName()),
	})
	if diags.HasError() {
		return Bot{}, diags
	}

	sqn := scopes.QualifiedName{Name: bot.GetMetadata().GetName(), Scope: bot.GetScope()}

	result := Bot{
		// User-provided attributes. Will be marked as null based on whether the
		// user provided legacy or RFD 153-style attributes.
		Metadata: types.ObjectNull(attrTypes("metadata")),
		Spec:     types.ObjectNull(attrTypes("spec")),
		Scope:    stringValue(bot.GetScope()),

		// Deprecated user-provided attributes.
		Name: types.StringNull(),

		// Computed attributes.
		ID:      stringValue(sqn.String()),
		Kind:    stringValue(bot.GetKind()),
		SubKind: stringValue(bot.GetSubKind()),
		Version: stringValue(bot.GetVersion()),
		Status:  statusValue,

		// Deprecated computed attributes. We still set them for backward-compatibility.
		UserName: stringValue(bot.GetStatus().GetUserName()),
		RoleName: stringValue(bot.GetStatus().GetRoleName()),
		TokenID:  base.TokenID,
	}

	if base.TTL.IsUnknown() {
		result.TTL = types.StringNull()
	} else {
		result.TTL = base.TTL
	}

	// If the plan or state includes the metadata or spec attribute, it means
	// the user has "opted in" to the new RFD 153-style attributes. Otherwise,
	// for backward-compatibility we'll populate the old fields.
	rfd153Style := importState || attrPresent(base.Metadata) || attrPresent(base.Spec) || attrPresent(base.Scope)

	if rfd153Style {
		result.Name = types.StringNull()

		var labels types.Map
		if len(bot.GetMetadata().GetLabels()) == 0 && attrNull(base.Metadata, "labels") {
			labels = types.MapNull(types.StringType)
		} else {
			elements := make(map[string]attr.Value)
			for k, v := range bot.GetMetadata().GetLabels() {
				elements[k] = types.StringValue(v)
			}

			labels, diags = types.MapValue(types.StringType, elements)
			if diags.HasError() {
				return Bot{}, diags
			}
		}

		result.Metadata, diags = types.ObjectValue(attrTypes("metadata"), map[string]attr.Value{
			"description": stringValue(bot.GetMetadata().GetDescription()),
			"expires":     timeValue(bot.GetMetadata().GetExpires()),
			"name":        stringValue(bot.GetMetadata().GetName()),
			"namespace":   stringValue(bot.GetMetadata().GetNamespace()),
			"revision":    stringValue(bot.GetMetadata().GetRevision()),
			"labels":      labels,
		})
		if diags.HasError() {
			return Bot{}, diags
		}

	} else {
		result.Metadata = types.ObjectNull(attrTypes("metadata"))
		result.Name = types.StringValue(bot.GetMetadata().GetName())
	}

	// If the traits list is empty, check the plan or state for whether we
	// should treat the attribute as null or an empty list.
	traitsNull := (!rfd153Style && base.Traits.IsNull()) ||
		(rfd153Style && attrNull(base.Spec, "traits"))

	var traits types.Map
	traitsElemType := types.ListType{
		ElemType: types.StringType,
	}
	if len(bot.GetSpec().GetTraits()) == 0 && traitsNull {
		traits = types.MapNull(traitsElemType)
	} else {
		elements := make(map[string]attr.Value)
		for _, trait := range bot.GetSpec().GetTraits() {
			elements[trait.GetName()], diags = types.ListValue(types.StringType,
				slices.Map(trait.GetValues(), func(s string) attr.Value {
					return types.StringValue(s)
				}),
			)
			if diags.HasError() {
				return Bot{}, diags
			}
		}
		traits, diags = types.MapValue(traitsElemType, elements)
		if diags.HasError() {
			return Bot{}, diags
		}
	}

	if rfd153Style {
		var roles types.List
		if len(bot.GetSpec().GetRoles()) == 0 && attrNull(base.Spec, "roles") {
			roles = types.ListNull(types.StringType)
		} else {
			roles, diags = types.ListValue(types.StringType,
				slices.Map(bot.GetSpec().GetRoles(), func(s string) attr.Value {
					return types.StringValue(s)
				}),
			)
			if diags.HasError() {
				return Bot{}, diags
			}
		}

		result.Spec, diags = types.ObjectValue(attrTypes("spec"), map[string]attr.Value{
			"traits":          traits,
			"roles":           roles,
			"max_session_ttl": durationValue(bot.GetSpec().GetMaxSessionTtl()),
		})
		if diags.HasError() {
			return Bot{}, diags
		}

		// Duration values can have different equivalent string representations
		// that cause Terraform to believe state has drifted or changed between
		// plan and apply (e.g. "5m" vs "5m0s").
		//
		// This is already handled by DurationValue.ToTerraformValue, but it
		// relies on storing the raw string in an unexported struct member, so
		// we restore the original value if it's equivalent to what the server
		// returned.
		if attrPresent(base.Spec) {
			if prev, ok := base.Spec.Attributes()["max_session_ttl"]; ok {
				// Scoped bots don't have a max_session_ttl. We don't want to replace the new value by the old one
				// if the old one is Unknown.
				if !prev.IsUnknown() {
					if dur, ok := prev.(tfschema.DurationValue); ok {
						if dur.Value == bot.GetSpec().GetMaxSessionTtl().AsDuration() {
							result.Spec.Attributes()["max_session_ttl"] = prev
						}
					}
				}
			}
		}

		result.Traits = types.MapNull(types.ListType{ElemType: types.StringType})
	} else {
		result.Spec = types.ObjectNull(attrTypes("spec"))
		result.Traits = traits

		if len(bot.GetSpec().GetRoles()) != 0 || base.Roles != nil {
			result.Roles = slices.Map(bot.GetSpec().GetRoles(), func(s string) types.String {
				return types.StringValue(s)
			})
		}
	}

	return result, nil
}

func (r resourceTeleportBot) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !r.p.IsConfigured(resp.Diagnostics) {
		return
	}

	var plan Bot
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rsp, err := r.p.Client().BotServiceClient().
		UpsertBot(ctx, &machineidv1.UpsertBotRequest{Bot: plan.ToProto()})
	if err != nil {
		resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr("Error updating Bot", err, "bot"))
		return
	}

	plan, diags = r.botFromProto(ctx, rsp, plan, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceTeleportBot) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state Bot
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.p.Client().BotServiceClient().
		DeleteBot(ctx, &machineidv1.DeleteBotRequest{
			BotName: state.GetName(),
			Scope:   state.GetScope(),
		})
	if err != nil {
		resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr("Error deleting Bot", trace.Wrap(err), "bot"))
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r resourceTeleportBot) ImportState(ctx context.Context, req resource.ImportStateRequest, rsp *resource.ImportStateResponse) {
	sqn, err := tfdriver.NewPossiblyUnscopedScopeQualifiedNameIdentifier(req.ID)
	if err != nil {
		rsp.Diagnostics.AddError("Error parsing bot ID", err.Error())
		return
	}

	bot, err := r.p.Client().BotServiceClient().
		GetBot(ctx, &machineidv1.GetBotRequest{
			BotName: sqn.Name,
			Scope:   sqn.Scope,
		})
	if err != nil {
		rsp.Diagnostics.Append(tfdiag.DiagFromWrappedErr("Error reading Bot", trace.Wrap(err), "bot"))
		return
	}

	state, diags := r.botFromProto(ctx, bot, Bot{}, true)
	rsp.Diagnostics.Append(diags...)
	if rsp.Diagnostics.HasError() {
		return
	}

	diags = rsp.State.Set(ctx, state)
	rsp.Diagnostics.Append(diags...)
	if rsp.Diagnostics.HasError() {
		return
	}
}

// rfd153OnlyValidator is used to ensure the user doesn't provide a mix of old
// and new style attributes (e.g. `spec.roles` and `<root>.traits`).
type rfd153OnlyValidator struct{}

func (rfd153OnlyValidator) Description(context.Context) string {
	return "Checks that deprecated attributes are not mixed and matched with their replacements"
}

func (v rfd153OnlyValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v rfd153OnlyValidator) Validate(ctx context.Context, req tfsdk.ValidateAttributeRequest, rsp *tfsdk.ValidateAttributeResponse) {
	if !attrPresent(req.AttributeConfig) {
		return
	}

	var meta, spec types.Object
	var scope types.String
	req.Config.GetAttribute(ctx, path.Root("metadata"), &meta)
	req.Config.GetAttribute(ctx, path.Root("spec"), &spec)
	req.Config.GetAttribute(ctx, path.Root("scope"), &scope)

	if attrPresent(meta) || attrPresent(spec) || attrPresent(scope) {
		rsp.Diagnostics.AddAttributeError(
			req.AttributePath,
			"Attribute Validation Error",
			fmt.Sprintf("The deprecated `%s` attribute cannot be used in combination with `spec`, `metadata`, or `scope`.", req.AttributePath),
		)
	}
}

// requiredUnlessLegacyValidator makes the attribute required *unless* the user
// provided a deprecated/legacy attribute instead (i.e. because they haven't yet
// migrated their configuration).
type requiredUnlessLegacyValidator struct{}

func (requiredUnlessLegacyValidator) Description(context.Context) string {
	return "Marks attributes required unless an equivalent legacy/deprecated attribute is provided"
}

func (v requiredUnlessLegacyValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v requiredUnlessLegacyValidator) Validate(ctx context.Context, req tfsdk.ValidateAttributeRequest, rsp *tfsdk.ValidateAttributeResponse) {
	if attrPresent(req.AttributeConfig) {
		return
	}

	var legacyConfig bool
	for _, name := range []string{"name", "roles", "traits"} {
		var attr attr.Value
		req.Config.GetAttribute(ctx, path.Root(name), &attr)
		if attrPresent(attr) {
			legacyConfig = true
			break
		}
	}

	if !legacyConfig {
		rsp.Diagnostics.AddAttributeError(
			req.AttributePath,
			"Missing required argument",
			fmt.Sprintf("The argument `%s` is required, but not definition was found.", req.AttributePath),
		)
	}
}

var (
	_ tfsdk.AttributeValidator = rfd153OnlyValidator{}
	_ tfsdk.AttributeValidator = requiredUnlessLegacyValidator{}
)

// Bot is the Terraform (i.e. state or plan) representation of a bot.
type Bot struct {
	ID types.String `tfsdk:"id"`

	Kind    types.String `tfsdk:"kind"`
	SubKind types.String `tfsdk:"sub_kind"`
	Version types.String `tfsdk:"version"`

	Metadata types.Object `tfsdk:"metadata"`
	Spec     types.Object `tfsdk:"spec"`
	Status   types.Object `tfsdk:"status"`
	Scope    types.String `tfsdk:"scope"`

	// Deprecated fields
	Name     types.String   `tfsdk:"name"`
	Roles    []types.String `tfsdk:"roles"`
	TokenID  types.String   `tfsdk:"token_id"`
	Traits   types.Map      `tfsdk:"traits"`
	TTL      types.String   `tfsdk:"token_ttl"`
	UserName types.String   `tfsdk:"user_name"`
	RoleName types.String   `tfsdk:"role_name"`
}

func (b Bot) ToProto() *machineidv1.Bot {
	return &machineidv1.Bot{
		Kind:     apitypes.KindBot,
		Version:  apitypes.V1,
		Metadata: b.GetMetadata(),
		Spec:     b.GetSpec(),
		Scope:    b.GetScope(),
	}
}

func (b Bot) GetMetadata() *headerv1.Metadata {
	return &headerv1.Metadata{
		Name:        b.GetName(),
		Description: b.GetDescription(),
		Expires:     b.GetExpires(),
		Namespace:   b.GetNamespace(),
		Revision:    b.GetRevision(),
		Labels:      b.GetLabels(),
	}
}

func (b Bot) GetSpec() *machineidv1.BotSpec {
	return &machineidv1.BotSpec{
		Traits:        b.GetTraits(),
		Roles:         b.GetRoles(),
		MaxSessionTtl: b.GetMaxSessionTTL(),
	}
}

func (b Bot) GetName() string {
	if attrPresent(b.Name) {
		return b.Name.ValueString()
	} else {
		if nm, ok := b.Metadata.Attributes()["name"]; ok {
			if str, ok := nm.(types.String); ok {
				return str.ValueString()
			}
		}
	}
	return ""
}

func (b Bot) GetDescription() string {
	if desc, ok := b.Metadata.Attributes()["description"]; ok {
		if str, ok := desc.(types.String); ok {
			return str.ValueString()
		}
	}
	return ""
}

func (b Bot) GetExpires() *timestamppb.Timestamp {
	if exp, ok := b.Metadata.Attributes()["expires"]; ok {
		if tv, ok := exp.(tfschema.TimeValue); ok {
			return timestamppb.New(tv.Value)
		}
	}
	return nil
}

func (b Bot) GetNamespace() string {
	if ns, ok := b.Metadata.Attributes()["namespace"]; ok {
		if str, ok := ns.(types.String); ok {
			return str.ValueString()
		}
	}
	return ""
}

func (b Bot) GetRevision() string {
	if rev, ok := b.Metadata.Attributes()["revision"]; ok {
		if str, ok := rev.(types.String); ok {
			return str.ValueString()
		}
	}
	return ""
}

func (b Bot) GetLabels() map[string]string {
	if !attrPresent(b.Metadata) {
		return nil
	}

	lbs, ok := b.Metadata.Attributes()["labels"]
	if !ok {
		return nil
	}

	mp, ok := lbs.(types.Map)
	if !ok {
		return nil
	}

	labels := make(map[string]string, len(mp.Elements()))
	for key, val := range mp.Elements() {
		if str, ok := val.(types.String); ok {
			labels[key] = str.ValueString()
		}
	}
	return labels
}

func (b Bot) GetTraits() []*machineidv1.Trait {
	var traitMap map[string]attr.Value
	if attrPresent(b.Spec) {
		if ts, ok := b.Spec.Attributes()["traits"]; ok && attrPresent(ts) {
			if mp, ok := ts.(types.Map); ok {
				traitMap = mp.Elements()
			}
		}
	} else if attrPresent(b.Traits) {
		traitMap = b.Traits.Elements()
	}

	if traitMap == nil {
		return nil
	}

	traits := make([]*machineidv1.Trait, 0, len(traitMap))
	for name, val := range traitMap {
		list, ok := val.(types.List)
		if !ok {
			continue
		}
		values := slices.FilterMapUnique(list.Elements(), func(v attr.Value) (string, bool) {
			if str, ok := v.(types.String); ok {
				return str.ValueString(), true
			}
			return "", false
		})
		traits = append(traits, &machineidv1.Trait{
			Name:   name,
			Values: values,
		})
	}
	return traits
}

func (b Bot) GetRoles() []string {
	if attrPresent(b.Spec) {
		if rs, ok := b.Spec.Attributes()["roles"]; ok {
			if list, ok := rs.(types.List); ok {
				return slices.FilterMapUnique(list.Elements(), func(v attr.Value) (string, bool) {
					if str, ok := v.(types.String); ok {
						return str.ValueString(), true
					}
					return "", false
				})
			}
		}
		return nil
	}

	return slices.Map(b.Roles, func(v types.String) string {
		return v.ValueString()
	})
}

func (b Bot) GetMaxSessionTTL() *durationpb.Duration {
	if !attrPresent(b.Spec) {
		return nil
	}

	ttl, ok := b.Spec.Attributes()["max_session_ttl"]
	if !ok {
		return nil
	}

	dur, ok := ttl.(tfschema.DurationValue)
	if !ok {
		return nil
	}

	return durationpb.New(dur.Value)
}

func (b Bot) GetScope() string {
	if !attrPresent(b.Scope) {
		return ""
	}
	return b.Scope.ValueString()
}

func attrPresent(v attr.Value) bool {
	return !v.IsNull() && !v.IsUnknown()
}
