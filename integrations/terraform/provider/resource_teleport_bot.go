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

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils/slices"

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
				PlanModifiers: []tfsdk.AttributePlanModifier{tfsdk.UseStateForUnknown()},
				Type:          types.StringType,
			},
			"kind": {
				Computed:      true,
				Description:   "The kind of resource represented.",
				Optional:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{tfsdk.UseStateForUnknown()},
				Type:          types.StringType,
			},
			"sub_kind": {
				Computed:      true,
				Description:   "Differentiates variations of the same kind. All resources should contain one, even if it is never populated.",
				Optional:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{tfsdk.UseStateForUnknown()},
				Type:          types.StringType,
			},
			"version": {
				Computed:      true,
				Description:   "The version of the resource being represented.",
				Optional:      true,
				PlanModifiers: []tfsdk.AttributePlanModifier{tfsdk.UseStateForUnknown()},
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
						PlanModifiers: []tfsdk.AttributePlanModifier{tfsdk.RequiresReplace()},
						Required:      true,
						Type:          types.StringType,
					},
					"namespace": {
						Computed:      true,
						Description:   "Namespace is object namespace. The field should be called \"namespace\" when it returns in Teleport 2.4.",
						Optional:      true,
						PlanModifiers: []tfsdk.AttributePlanModifier{tfsdk.UseStateForUnknown()},
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

			// Deprecated fields.
			"name": {
				Type:               types.StringType,
				Optional:           true,
				Description:        "The name of the bot, i.e. the unprefixed User name",
				DeprecationMessage: "Deprecated. Used `metadata.name` instead.",
				Validators: []tfsdk.AttributeValidator{
					rfd153OnlyValidator{},
				},
				PlanModifiers: []tfsdk.AttributePlanModifier{tfsdk.RequiresReplace()},
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

// resourceTeleportBotType is the resource metadata type
type resourceTeleportBotType struct{}

// GetSchema returns the resource schema
func (r resourceTeleportBotType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	// It's unusual for this provider, but we'll hand-write the schema here as
	// bots do not have any server-side resources of their own.
	return GenSchemaBot(ctx)
}

// NewResource creates the empty resource
func (r resourceTeleportBotType) NewResource(_ context.Context, p tfsdk.Provider) (tfsdk.Resource, diag.Diagnostics) {
	return resourceTeleportBot{
		p: *(p.(*Provider)),
	}, nil
}

// resourceTeleportBot is the resource
type resourceTeleportBot struct {
	p Provider
}

func (r resourceTeleportBot) Create(ctx context.Context, req tfsdk.CreateResourceRequest, resp *tfsdk.CreateResourceResponse) {
	if !r.p.IsConfigured(resp.Diagnostics) {
		return
	}

	var plan Bot
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	response, err := r.p.Client.BotServiceClient().
		CreateBot(ctx, &machineidv1.CreateBotRequest{Bot: plan.ToProto()})
	if err != nil {
		resp.Diagnostics.Append(diagFromWrappedErr("Error creating Bot", trace.Wrap(err), "bot"))
		return
	}

	diags = resp.State.Set(ctx, r.botFromProto(ctx, response, plan))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceTeleportBot) Read(ctx context.Context, req tfsdk.ReadResourceRequest, resp *tfsdk.ReadResourceResponse) {
	var state Bot
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	bot, err := r.p.Client.BotServiceClient().GetBot(ctx, &machineidv1.GetBotRequest{
		BotName: state.GetName(),
	})
	switch {
	case trace.IsNotFound(err):
		resp.State.RemoveResource(ctx)
		return
	case err != nil:
		resp.Diagnostics.Append(diagFromWrappedErr("Error reading Bot", trace.Wrap(err), "bot"))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, r.botFromProto(ctx, bot, state))...)
}

// botFromProto converts the server/protobuf representation of a bot to Terraform
// using the given "base" (plan or state) to determine whether to fill the legacy
// or new attributes.
//
// NOTE: Terraform is *very* fussy about non-computed attributes changing between
// plan and apply, even in benign ways such as null becoming an empty list, so
// exercise caution!
func (r resourceTeleportBot) botFromProto(ctx context.Context, bot *machineidv1.Bot, base Bot) Bot {
	schema, _ := GenSchemaBot(ctx)

	attrTypes := func(key string) map[string]attr.Type {
		result := make(map[string]attr.Type)
		for k, v := range schema.Attributes[key].Attributes.GetAttributes() {
			result[k] = v.Type
		}
		return result
	}

	stringValue := func(v string) types.String {
		return types.String{Value: v, Null: v == ""}
	}

	timeValue := func(v *timestamppb.Timestamp) tfschema.TimeValue {
		if v == nil {
			return tfschema.TimeValue{Null: true}
		}
		return tfschema.TimeValue{
			Value:  v.AsTime(),
			Format: time.RFC3339,
		}
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
		if obj.Attrs == nil {
			return true
		}
		attr, ok := obj.Attrs[name]
		return !ok || attr.IsNull()
	}

	result := Bot{
		// User-provided attributes. Will be marked as null based on whether the
		// user provided legacy or RFD 153-style attributes.
		Metadata: types.Object{AttrTypes: attrTypes("metadata")},
		Spec:     types.Object{AttrTypes: attrTypes("spec")},

		// Deprecated user-provided attributes.
		Name: types.String{},

		// Computed attributes.
		ID:      stringValue(bot.GetMetadata().GetName()),
		Kind:    stringValue(bot.GetKind()),
		SubKind: stringValue(bot.GetSubKind()),
		Version: stringValue(bot.GetVersion()),
		Status: types.Object{
			AttrTypes: attrTypes("status"),
			Attrs: map[string]attr.Value{
				"user_name": stringValue(bot.GetStatus().GetUserName()),
				"role_name": stringValue(bot.GetStatus().GetRoleName()),
			},
		},

		// Deprecated computed attributes. We still set them for backward-compatibility.
		UserName: stringValue(bot.GetStatus().GetUserName()),
		RoleName: stringValue(bot.GetStatus().GetRoleName()),
		TokenID:  base.TokenID,
	}

	if base.TTL.Unknown {
		result.TTL = types.String{Null: true}
	} else {
		result.TTL = base.TTL
	}

	// If the plan or state includes the metadata or spec attribute, it means
	// the user has "opted in" to the new RFD 153-style attributes. Otherwise,
	// for backward-compatibility we'll populate the old fields.
	rfd153Style := attrPresent(base.Metadata) || attrPresent(base.Spec)

	if rfd153Style {
		result.Name.Null = true

		labels := types.Map{
			Elems:    make(map[string]attr.Value),
			ElemType: types.StringType,
			Null:     len(bot.GetMetadata().GetLabels()) == 0 && attrNull(base.Metadata, "labels"),
		}
		for k, v := range bot.GetMetadata().GetLabels() {
			labels.Elems[k] = types.String{Value: v}
		}

		result.Metadata.Attrs = map[string]attr.Value{
			"description": stringValue(bot.GetMetadata().GetDescription()),
			"expires":     timeValue(bot.GetMetadata().GetExpires()),
			"name":        stringValue(bot.GetMetadata().GetName()),
			"namespace":   stringValue(bot.GetMetadata().GetNamespace()),
			"revision":    stringValue(bot.GetMetadata().GetRevision()),
			"labels":      labels,
		}
	} else {
		result.Metadata.Null = true
		result.Name.Value = bot.GetMetadata().GetName()
	}

	// If the traits list is empty, check the plan or state for whether we
	// should treat the attribute as null or an empty list.
	traitsNull := (!rfd153Style && base.Traits.IsNull()) ||
		(rfd153Style && attrNull(base.Spec, "traits"))

	traits := types.Map{
		Null:  len(bot.GetSpec().GetTraits()) == 0 && traitsNull,
		Elems: map[string]attr.Value{},
		ElemType: types.ListType{
			ElemType: types.StringType,
		},
	}
	for _, trait := range bot.GetSpec().GetTraits() {
		traits.Elems[trait.GetName()] = types.List{
			Elems: slices.Map(trait.GetValues(), func(s string) attr.Value {
				return types.String{Value: s}
			}),
			ElemType: types.StringType,
		}
	}

	if rfd153Style {
		result.Spec.Attrs = map[string]attr.Value{
			"traits": traits,
			"roles": types.List{
				Elems: slices.Map(bot.GetSpec().GetRoles(), func(s string) attr.Value {
					return types.String{Value: s}
				}),
				ElemType: types.StringType,
				Null:     len(bot.GetSpec().GetRoles()) == 0 && attrNull(base.Spec, "roles"),
			},
			"max_session_ttl": durationValue(bot.GetSpec().GetMaxSessionTtl()),
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
			if prev, ok := base.Spec.Attrs["max_session_ttl"]; ok {
				if dur, ok := prev.(tfschema.DurationValue); ok {
					if dur.Value == bot.GetSpec().GetMaxSessionTtl().AsDuration() {
						result.Spec.Attrs["max_session_ttl"] = prev
					}
				}
			}
		}

		result.Traits = types.Map{
			Null:     true,
			ElemType: types.ListType{ElemType: types.StringType},
		}
	} else {
		result.Spec.Null = true
		result.Traits = traits

		if len(bot.GetSpec().GetRoles()) != 0 || base.Roles != nil {
			result.Roles = slices.Map(bot.GetSpec().GetRoles(), func(s string) types.String {
				return types.String{Value: s}
			})
		}
	}

	return result
}

func (r resourceTeleportBot) Update(ctx context.Context, req tfsdk.UpdateResourceRequest, resp *tfsdk.UpdateResourceResponse) {
	if !r.p.IsConfigured(resp.Diagnostics) {
		return
	}

	var plan Bot
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rsp, err := r.p.Client.BotServiceClient().
		UpsertBot(ctx, &machineidv1.UpsertBotRequest{Bot: plan.ToProto()})
	if err != nil {
		resp.Diagnostics.Append(diagFromWrappedErr("Error updating Bot", err, "bot"))
		return
	}

	diags = resp.State.Set(ctx, r.botFromProto(ctx, rsp, plan))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceTeleportBot) Delete(ctx context.Context, req tfsdk.DeleteResourceRequest, resp *tfsdk.DeleteResourceResponse) {
	var state Bot
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.p.Client.BotServiceClient().
		DeleteBot(ctx, &machineidv1.DeleteBotRequest{BotName: state.GetName()})
	if err != nil {
		resp.Diagnostics.Append(diagFromWrappedErr("Error deleting Bot", trace.Wrap(err), "bot"))
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r resourceTeleportBot) ImportState(ctx context.Context, req tfsdk.ImportResourceStateRequest, rsp *tfsdk.ImportResourceStateResponse) {
	bot, err := r.p.Client.BotServiceClient().
		GetBot(ctx, &machineidv1.GetBotRequest{BotName: req.ID})
	if err != nil {
		rsp.Diagnostics.Append(diagFromWrappedErr("Error reading Bot", trace.Wrap(err), "bot"))
		return
	}

	diags := rsp.State.Set(ctx, r.botFromProto(ctx, bot, Bot{}))
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
	req.Config.GetAttribute(ctx, path.Root("metadata"), &meta)
	req.Config.GetAttribute(ctx, path.Root("spec"), &spec)

	if attrPresent(meta) || attrPresent(spec) {
		rsp.Diagnostics.AddAttributeError(
			req.AttributePath,
			"Attribute Validation Error",
			fmt.Sprintf("The deprecated `%s` attribute cannot be used in combination with `spec` or `metadata`.", req.AttributePath),
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
		return b.Name.Value
	} else {
		if nm, ok := b.Metadata.Attrs["name"]; ok {
			if str, ok := nm.(types.String); ok {
				return str.Value
			}
		}
	}
	return ""
}

func (b Bot) GetDescription() string {
	if desc, ok := b.Metadata.Attrs["description"]; ok {
		if str, ok := desc.(types.String); ok {
			return str.Value
		}
	}
	return ""
}

func (b Bot) GetExpires() *timestamppb.Timestamp {
	if exp, ok := b.Metadata.Attrs["expires"]; ok {
		if tv, ok := exp.(tfschema.TimeValue); ok {
			return timestamppb.New(tv.Value)
		}
	}
	return nil
}

func (b Bot) GetNamespace() string {
	if ns, ok := b.Metadata.Attrs["namespace"]; ok {
		if str, ok := ns.(types.String); ok {
			return str.Value
		}
	}
	return ""
}

func (b Bot) GetRevision() string {
	if rev, ok := b.Metadata.Attrs["revision"]; ok {
		if str, ok := rev.(types.String); ok {
			return str.Value
		}
	}
	return ""
}

func (b Bot) GetLabels() map[string]string {
	if !attrPresent(b.Metadata) {
		return nil
	}

	lbs, ok := b.Metadata.Attrs["labels"]
	if !ok {
		return nil
	}

	mp, ok := lbs.(types.Map)
	if !ok {
		return nil
	}

	labels := make(map[string]string, len(mp.Elems))
	for key, val := range mp.Elems {
		if str, ok := val.(types.String); ok {
			labels[key] = str.Value
		}
	}
	return labels
}

func (b Bot) GetTraits() []*machineidv1.Trait {
	var traitMap map[string]attr.Value
	if attrPresent(b.Spec) {
		if ts, ok := b.Spec.Attrs["traits"]; ok && attrPresent(ts) {
			if mp, ok := ts.(types.Map); ok {
				traitMap = mp.Elems
			}
		}
	} else if attrPresent(b.Traits) {
		traitMap = b.Traits.Elems
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
		values := slices.FilterMapUnique(list.Elems, func(v attr.Value) (string, bool) {
			if str, ok := v.(types.String); ok {
				return str.Value, true
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
		if rs, ok := b.Spec.Attrs["roles"]; ok {
			if list, ok := rs.(types.List); ok {
				return slices.FilterMapUnique(list.Elems, func(v attr.Value) (string, bool) {
					if str, ok := v.(types.String); ok {
						return str.Value, true
					}
					return "", false
				})
			}
		}
		return nil
	}

	return slices.Map(b.Roles, func(v types.String) string {
		return v.Value
	})
}

func (b Bot) GetMaxSessionTTL() *durationpb.Duration {
	if !attrPresent(b.Spec) {
		return nil
	}

	ttl, ok := b.Spec.Attrs["max_session_ttl"]
	if !ok {
		return nil
	}

	dur, ok := ttl.(tfschema.DurationValue)
	if !ok {
		return nil
	}

	return durationpb.New(dur.Value)
}

func attrPresent(v attr.Value) bool {
	return !v.IsNull() && !v.IsUnknown()
}
