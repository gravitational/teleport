package loginrule

import (
	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"

	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/utils"
)

// ResourceKind is the value used to identify login rules in the resource
// header.
const ResourceKind = "login_rule"

// loginRuleResource is a type to represent login rules which implements types.loginRuleResource
// and custom YAML (un)marshaling. This satisfies the expected YAML format for
// the resource, which would be hard/impossible to do for loginrulepb.LoginRule
// directly. Specifically, protoc-gen-go does not have good support for parsing
// a map[string][]string from YAML.
type loginRuleResource struct {
	// ResourceHeader is embedded to implement types.Resource
	types.ResourceHeader
	// Spec is the login rule specification
	Spec loginRuleSpec `json:"spec"`
}

// loginRuleSpec holds the login rule properties.
type loginRuleSpec struct {
	Priority         int32               `json:"priority"`
	TraitsMap        map[string][]string `json:"traits_map,omitempty"`
	TraitsExpression string              `json:"traits_expression,omitempty"`
}

// CheckAndSetDefaults sanity Resource fields to catch simple errors, and sets
// default values for all fields with defaults.
func (r *loginRuleResource) CheckAndSetDefaults() error {
	if err := r.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if r.Kind == "" {
		r.Kind = ResourceKind
	} else if r.Kind != ResourceKind {
		return trace.BadParameter("unexpected resource kind %q, must be %q", r.Kind, ResourceKind)
	}
	if r.Version == "" {
		r.Version = "v1"
	} else if r.Version != "v1" {
		return trace.BadParameter("unsupported resource version %q, \"v1\" is currently the only supported version", r.Version)
	}
	if r.Metadata.Name == "" {
		return trace.BadParameter("login rule must have a name")
	}
	if len(r.Spec.TraitsMap) > 0 && len(r.Spec.TraitsExpression) > 0 {
		return trace.BadParameter("login rule has non-empty traits_map and traits_expression, exactly one must be set")
	}
	if len(r.Spec.TraitsMap) == 0 && len(r.Spec.TraitsExpression) == 0 {
		return trace.BadParameter("login rule has empty traits_map and traits_expression, exactly one must be set")
	}
	for key, values := range r.Spec.TraitsMap {
		empty := true
		for _, value := range values {
			if len(value) > 0 {
				empty = false
				break
			}
		}
		if empty {
			return trace.BadParameter("traits_map has zero non-empty values for key %q", key)
		}
	}
	return nil
}

func resourceToProto(r *loginRuleResource) *loginrulepb.LoginRule {
	return &loginrulepb.LoginRule{
		Metadata:         proto.Clone(&r.Metadata).(*types.Metadata),
		Version:          r.Version,
		Priority:         r.Spec.Priority,
		TraitsMap:        traitsMapResourceToProto(r.Spec.TraitsMap),
		TraitsExpression: r.Spec.TraitsExpression,
	}
}

func traitsMapResourceToProto(in map[string][]string) map[string]*wrappers.StringValues {
	if in == nil {
		return nil
	}
	out := make(map[string]*wrappers.StringValues, len(in))
	for key, values := range in {
		out[key] = &wrappers.StringValues{
			Values: append([]string{}, values...),
		}
	}
	return out
}

// unmarshalLoginRule parses a login rule in the Resource format which matches the
// expected YAML format for Teleport resources, sets default values, and
// converts to *loginrulepb.LoginRule.
func unmarshalLoginRule(raw []byte) (*loginrulepb.LoginRule, error) {
	var resource loginRuleResource
	if err := utils.FastUnmarshal(raw, &resource); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := resource.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return resourceToProto(&resource), nil
}
