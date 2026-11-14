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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
func GenSchemaBot(ctx context.Context) (schema.Schema, diag.Diagnostics) {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
			},
			"kind": schema.StringAttribute{
				Computed:      true,
				Description:   "The kind of resource represented.",
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
			},
			"sub_kind": schema.StringAttribute{
				Computed:      true,
				Description:   "Differentiates variations of the same kind. All resources should contain one, even if it is never populated.",
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
			},
			"version": schema.StringAttribute{
				Computed:      true,
				Description:   "The version of the resource being represented.",
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
				Validators:    []validator.String{tfschema.UseVersionBetween(1, 1)},
			},
			"metadata": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"description": schema.StringAttribute{
						Description: "Description is object description",
						Optional:    true,
					},
					"expires": schema.StringAttribute{
						Description: "Expires is a global expiry time header can be set on any resource in the system.",
						Optional:    true,
						CustomType:  tfschema.UseRFC3339Time(),
						Validators:  []validator.String{tfschema.MustTimeBeInFuture()},
					},
					"labels": schema.MapAttribute{
						Description: "Labels is a set of labels",
						Optional:    true,
						ElementType: types.StringType,
						Validators:  []validator.Map{tfschema.UseMapKeysPresentValidator("teleport.dev/origin")},
					},
					"name": schema.StringAttribute{
						Description:   "Name is an object name",
						PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
						Required:      true,
					},
					"namespace": schema.StringAttribute{
						Computed:      true,
						Description:   "Namespace is object namespace. The field should be called \"namespace\" when it returns in Teleport 2.4.",
						Optional:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
					},
					"revision": schema.StringAttribute{
						Description: "Revision is an opaque identifier which tracks the versions of a resource over time. Clients should ignore and not alter its value but must return the revision in any updates of a resource.",
						Optional:    true,
					},
				},
				Description: "Common metadata that all resources share",
				Optional:    true,
				Validators: []validator.Object{
					requiredUnlessLegacyValidator{},
				},
			},
			"spec": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"traits": tfschema.GenSchemaTraitsResource(ctx, schema.MapAttribute{
						Description: "The traits that will be associated with the bot for the purposes of role templating.\n\nWhere multiple specified with the same name, these will be merged by the server.",
					}),
					"roles": schema.ListAttribute{
						Optional:    true,
						Description: "A list of roles the created bot should be allowed to assume via role impersonation.",
						ElementType: types.StringType,
					},
					"max_session_ttl": schema.StringAttribute{
						Optional:    true,
						Computed:    true,
						Description: "The max session TTL value for the bot's internal role. Unless specified, bots may not request a value beyond the default maximum TTL of 12 hours. This value may not be larger than 7 days (168 hours).",
						CustomType:  tfschema.DurationType{},
					},
					"direct_unscoped_role_assignment": schema.BoolAttribute{
						Optional:      true,
						Computed:      true,
						Description:   "Determines whether unscoped roles are assigned directly to the bot's internal user, rather than indirectly via role impersonation. Defaults to false.",
						PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
					},
				},
				Description: "The configured properties of a bot.",
				Optional:    true,
				Validators: []validator.Object{
					requiredUnlessLegacyValidator{},
				},
			},
			"status": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"user_name": schema.StringAttribute{
						Description: "The name of the user associated with the bot.",
						Computed:    true,
					},
					"role_name": schema.StringAttribute{
						Description: "The name of the role associated with the bot.",
						Computed:    true,
					},
				},
				Description: "Fields that are set by the server as results of operations. These should not be modified by users.",
				Computed:    true,
			},
			"scope": schema.StringAttribute{
				Description:   "Scope is the scope of the bot resource. Leave empty for unscoped bots.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Optional:      true,
			},

			// Deprecated fields.
			"name": schema.StringAttribute{
				Optional:           true,
				Description:        "The name of the bot, i.e. the unprefixed User name",
				DeprecationMessage: "Deprecated. Used `metadata.name` instead.",
				Validators: []validator.String{
					rfd153OnlyValidator{},
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"user_name": schema.StringAttribute{
				Computed:           true,
				Description:        "The name of the generated bot user",
				DeprecationMessage: "Deprecated. Use `status.user_name` instead.",
			},
			"role_name": schema.StringAttribute{
				Computed:           true,
				Description:        "The name of the generated bot role",
				DeprecationMessage: "Deprecated. Use `status.role_name` instead.",
			},
			"token_ttl": schema.StringAttribute{
				Optional:           true,
				Computed:           true,
				DeprecationMessage: "Deprecated. This field is not required anymore and has no effect.",
			},
			"token_id": schema.StringAttribute{
				// Implementation note: this is not used anymore, we can skip this
				// This will go away eventually when we'll generate the bot provider instead
				Optional:           true,
				Sensitive:          true,
				DeprecationMessage: "Deprecated. This field is not required anymore and has no effect.",
			},
			"roles": schema.ListAttribute{
				ElementType:        types.StringType,
				Optional:           true,
				Description:        "A list of roles the created bot should be allowed to assume via role impersonation.",
				DeprecationMessage: "Deprecated. Use `spec.roles` instead.",
				Validators: []validator.List{
					rfd153OnlyValidator{},
				},
			},
			"traits": tfschema.GenSchemaTraitsResource(ctx, schema.MapAttribute{
				DeprecationMessage: "Deprecated. Use `spec.traits` instead.",
				Validators: []validator.Map{
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

// Schema returns the resource schema
func (r resourceTeleportBot) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	// It's unusual for this provider, but we'll hand-write the schema here as
	// bots do not have any server-side resources of their own.
	schema, diags := GenSchemaBot(ctx)
	resp.Diagnostics.Append(diags...)
	resp.Schema = schema
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
	botSchema, diags := GenSchemaBot(ctx)
	if diags.HasError() {
		return Bot{}, diags
	}

	attributes := botSchema.GetAttributes()
	nestedAttrTypes := func(key string) (map[string]attr.Type, diag.Diagnostic) {
		attribute, ok := attributes[key].(schema.NestedAttribute)
		if !ok {
			return nil, diag.NewErrorDiagnostic(
				"failed to get attribute types",
				fmt.Sprintf("expected nested attribute, got: %T", attributes[key]))
		}

		result := make(map[string]attr.Type)
		for k, v := range attribute.GetNestedObject().GetAttributes() {
			result[k] = v.GetType()
		}
		return result, nil
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

	statusTypes, errDiag := nestedAttrTypes("status")
	diags.Append(errDiag)
	if diags.HasError() {
		return Bot{}, diags
	}

	statusValue, diags := types.ObjectValue(statusTypes, map[string]attr.Value{
		"user_name": stringValue(bot.GetStatus().GetUserName()),
		"role_name": stringValue(bot.GetStatus().GetRoleName()),
	})
	if diags.HasError() {
		return Bot{}, diags
	}

	sqn := scopes.QualifiedName{Name: bot.GetMetadata().GetName(), Scope: bot.GetScope()}

	metadataTypes, errDiag := nestedAttrTypes("metadata")
	diags.Append(errDiag)
	if diags.HasError() {
		return Bot{}, diags
	}

	specTypes, errDiag := nestedAttrTypes("spec")
	diags.Append(errDiag)
	if diags.HasError() {
		return Bot{}, diags
	}

	result := Bot{
		// User-provided attributes. Will be marked as null based on whether the
		// user provided legacy or RFD 153-style attributes.
		Metadata: types.ObjectNull(metadataTypes),
		Spec:     types.ObjectNull(specTypes),
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

		result.Metadata, diags = types.ObjectValue(metadataTypes, map[string]attr.Value{
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
		result.Metadata = types.ObjectNull(metadataTypes)
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

		result.Spec, diags = types.ObjectValue(specTypes, map[string]attr.Value{
			"traits":                          traits,
			"roles":                           roles,
			"max_session_ttl":                 durationValue(bot.GetSpec().GetMaxSessionTtl()),
			"direct_unscoped_role_assignment": types.BoolValue(bot.GetSpec().GetDirectUnscopedRoleAssignment()),
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
							attrs := result.Spec.Attributes()
							attrs["max_session_ttl"] = prev
							result.Spec, diags = types.ObjectValue(result.Spec.AttributeTypes(ctx), attrs)
							if diags.HasError() {
								return Bot{}, diags
							}
						}
					}
				}
			}
		}

		result.Traits = types.MapNull(types.ListType{ElemType: types.StringType})
	} else {
		result.Spec = types.ObjectNull(specTypes)
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

func (v rfd153OnlyValidator) ValidateString(ctx context.Context, req validator.StringRequest, rsp *validator.StringResponse) {
	rsp.Diagnostics.Append(v.validate(ctx, req.Config, req.Path, req.ConfigValue)...)
}

func (v rfd153OnlyValidator) ValidateList(ctx context.Context, req validator.ListRequest, rsp *validator.ListResponse) {
	rsp.Diagnostics.Append(v.validate(ctx, req.Config, req.Path, req.ConfigValue)...)
}

func (v rfd153OnlyValidator) ValidateMap(ctx context.Context, req validator.MapRequest, rsp *validator.MapResponse) {
	rsp.Diagnostics.Append(v.validate(ctx, req.Config, req.Path, req.ConfigValue)...)
}

func (v rfd153OnlyValidator) validate(ctx context.Context, config tfsdk.Config, attrPath path.Path, configValue attr.Value) diag.Diagnostics {
	var diags diag.Diagnostics
	if !attrPresent(configValue) {
		return diags
	}
	var meta, spec types.Object
	var scope types.String
	diags.Append(config.GetAttribute(ctx, path.Root("metadata"), &meta)...)
	diags.Append(config.GetAttribute(ctx, path.Root("spec"), &spec)...)
	diags.Append(config.GetAttribute(ctx, path.Root("scope"), &scope)...)
	if diags.HasError() {
		return diags
	}

	if attrPresent(meta) || attrPresent(spec) || attrPresent(scope) {
		diags.AddAttributeError(
			attrPath,
			"Attribute Validation Error",
			fmt.Sprintf("The deprecated `%s` attribute cannot be used in combination with `spec`, `metadata`, or `scope`.", attrPath),
		)
	}

	return diags
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

func (v requiredUnlessLegacyValidator) ValidateObject(ctx context.Context, req validator.ObjectRequest, rsp *validator.ObjectResponse) {
	if attrPresent(req.ConfigValue) {
		return
	}

	var legacyConfig bool
	for _, name := range []string{"name", "roles", "traits"} {
		var attr attr.Value
		rsp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), &attr)...)
		if rsp.Diagnostics.HasError() {
			return
		}

		if attrPresent(attr) {
			legacyConfig = true
			break
		}
	}

	if !legacyConfig {
		rsp.Diagnostics.AddAttributeError(
			req.Path,
			"Missing required argument",
			fmt.Sprintf("The argument `%s` is required, but not definition was found.", req.Path),
		)
	}
}

var (
	_ validator.Map    = rfd153OnlyValidator{}
	_ validator.List   = rfd153OnlyValidator{}
	_ validator.String = rfd153OnlyValidator{}
	_ validator.Object = requiredUnlessLegacyValidator{}
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
		Traits:                       b.GetTraits(),
		Roles:                        b.GetRoles(),
		MaxSessionTtl:                b.GetMaxSessionTTL(),
		DirectUnscopedRoleAssignment: b.GetDirectUnscopedRoleAssignment(),
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

func (b Bot) GetDirectUnscopedRoleAssignment() bool {
	if !attrPresent(b.Spec) {
		return false
	}
	direct, ok := b.Spec.Attributes()["direct_unscoped_role_assignment"]
	if !ok {
		return false
	}
	bool, ok := direct.(types.Bool)
	if !ok {
		return false
	}
	return bool.ValueBool()
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
