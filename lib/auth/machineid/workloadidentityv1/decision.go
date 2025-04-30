// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package workloadidentityv1

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/structpb"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/auth/machineid/workloadidentityv1/expression"
	"github.com/gravitational/teleport/lib/utils"
)

type decision struct {
	templatedWorkloadIdentity *workloadidentityv1pb.WorkloadIdentity
	shouldIssue               bool
	reason                    error
	sigstorePolicyResults     map[string]error
}

func decide(
	ctx context.Context,
	wi *workloadidentityv1pb.WorkloadIdentity,
	attrs *workloadidentityv1pb.Attrs,
	sigstore SigstorePolicyEvaluator,
) *decision {
	d := &decision{
		templatedWorkloadIdentity: proto.Clone(wi).(*workloadidentityv1pb.WorkloadIdentity),
		sigstorePolicyResults:     make(map[string]error),
	}

	// First, evaluate the rules.
	err := evaluateRules(ctx, wi, attrs, sigstore, d.sigstorePolicyResults)
	if err != nil {
		d.reason = trace.Wrap(err, "attributes did not pass rule evaluation")
		return d
	}

	// Now we can cook up some templating...
	templated, err := expression.RenderTemplate(wi.GetSpec().GetSpiffe().GetId(), &expression.Environment{Attrs: attrs})
	if err != nil {
		d.reason = trace.Wrap(err, "templating spec.spiffe.id")
		return d
	}
	d.templatedWorkloadIdentity.Spec.Spiffe.Id = templated

	templated, err = expression.RenderTemplate(wi.GetSpec().GetSpiffe().GetHint(), &expression.Environment{Attrs: attrs})
	if err != nil {
		d.reason = trace.Wrap(err, "templating spec.spiffe.hint")
		return d
	}
	d.templatedWorkloadIdentity.Spec.Spiffe.Hint = templated

	for i, san := range wi.GetSpec().GetSpiffe().GetX509().GetDnsSans() {
		templated, err = expression.RenderTemplate(san, &expression.Environment{Attrs: attrs})
		if err != nil {
			d.reason = trace.Wrap(err, "templating spec.spiffe.x509.dns_sans[%d]", i)
			return d
		}
		if !utils.IsValidHostname(templated) {
			d.reason = trace.BadParameter(
				"templating spec.spiffe.x509.dns_sans[%d] resulted in an invalid DNS name %q",
				i,
				templated,
			)
			return d
		}
		d.templatedWorkloadIdentity.Spec.Spiffe.X509.DnsSans[i] = templated
	}

	st := wi.GetSpec().GetSpiffe().GetX509().GetSubjectTemplate()
	if st != nil {
		dst := d.templatedWorkloadIdentity.Spec.Spiffe.X509.SubjectTemplate

		templated, err = expression.RenderTemplate(st.CommonName, &expression.Environment{Attrs: attrs})
		if err != nil {
			d.reason = trace.Wrap(
				err,
				"templating spec.spiffe.x509.subject_template.common_name",
			)
			return d
		}
		dst.CommonName = templated

		templated, err = expression.RenderTemplate(st.Organization, &expression.Environment{Attrs: attrs})
		if err != nil {
			d.reason = trace.Wrap(
				err,
				"templating spec.spiffe.x509.subject_template.organization",
			)
			return d
		}
		dst.Organization = templated

		templated, err = expression.RenderTemplate(st.OrganizationalUnit, &expression.Environment{Attrs: attrs})
		if err != nil {
			d.reason = trace.Wrap(
				err,
				"templating spec.spiffe.x509.subject_template.organizational_unit",
			)
			return d
		}
		dst.OrganizationalUnit = templated
	}

	if ec := wi.GetSpec().GetSpiffe().GetJwt().GetExtraClaims(); ec != nil {
		templated, err := templateExtraClaims(ec, attrs)
		if err != nil {
			d.reason = trace.Wrap(
				err,
				"templating spec.spiffe.jwt.extra_claims",
			)
			return d
		}
		d.templatedWorkloadIdentity.Spec.Spiffe.Jwt.ExtraClaims = templated
	}

	// Yay - made it to the end!
	d.shouldIssue = true
	return d
}

// getFieldStringValue returns a string value from the given attribute set.
// The attribute is specified as a dot-separated path to the field in the
// attribute set.
//
// The specified attribute must be a string field. If the attribute is not
// found, an error is returned.
//
// TODO: convert rules into predicate expressions, so we can evaluate them
// using the expressions package (and remove this function).
func getFieldStringValue(attrs *workloadidentityv1pb.Attrs, attr string) (string, error) {
	attrParts := strings.Split(attr, ".")
	message := attrs.ProtoReflect()
	// TODO(noah): Improve errors by including the fully qualified attribute
	// (e.g add up the parts of the attribute path processed thus far)
	for i, part := range attrParts {
		fieldDesc := message.Descriptor().Fields().ByTextName(part)
		if fieldDesc == nil {
			return "", trace.NotFound("attribute %q not found", part)
		}
		// We expect the final key to point to a string field - otherwise - we
		// return an error.
		if i == len(attrParts)-1 {
			if !slices.Contains([]protoreflect.Kind{
				protoreflect.StringKind,
				protoreflect.BoolKind,
				protoreflect.Int32Kind,
				protoreflect.Int64Kind,
				protoreflect.Uint64Kind,
				protoreflect.Uint32Kind,
			}, fieldDesc.Kind()) {
				return "", trace.BadParameter("attribute %q of type %q cannot be converted to string", part, fieldDesc.Kind())
			}
			return message.Get(fieldDesc).String(), nil
		}
		// If we're not processing the final key part, we expect this to point
		// to a message that we can further explore.
		if fieldDesc.Kind() != protoreflect.MessageKind {
			return "", trace.BadParameter("attribute %q is not a message", part)
		}
		message = message.Get(fieldDesc).Message()
	}
	return "", nil
}

func evaluateRules(
	ctx context.Context,
	wi *workloadidentityv1pb.WorkloadIdentity,
	attrs *workloadidentityv1pb.Attrs,
	sigstore SigstorePolicyEvaluator,
	sigstorePolicyResults map[string]error,
) error {
	if len(wi.GetSpec().GetRules().GetAllow()) == 0 {
		return nil
	}

	sigstoreEvaluator := expression.SigstorePolicyEvaluatorFunc(func(policyNames []string) (bool, error) {
		// Evaluate policies we haven't already evaluated.
		var unevaluated []string
		for _, name := range policyNames {
			if _, evaluated := sigstorePolicyResults[name]; !evaluated {
				unevaluated = append(unevaluated, name)
			}
		}
		if len(unevaluated) != 0 {
			resultMap, err := sigstore.Evaluate(ctx, unevaluated, attrs)
			if err != nil {
				return false, err
			}
			for k, v := range resultMap {
				sigstorePolicyResults[k] = v
			}
		}

		// If any of them resulted in an error, return false.
		for _, name := range policyNames {
			if sigstorePolicyResults[name] != nil {
				return false, nil
			}
		}
		return true, nil
	})

ruleLoop:
	for _, rule := range wi.GetSpec().GetRules().GetAllow() {
		if rule.GetExpression() != "" {
			pass, err := expression.Evaluate(rule.GetExpression(), &expression.Environment{
				Attrs:                   attrs,
				SigstorePolicyEvaluator: sigstoreEvaluator,
			})
			if err != nil {
				return err
			}
			if pass {
				return nil
			} else {
				continue ruleLoop
			}
		}

		for _, condition := range rule.GetConditions() {
			val, err := getFieldStringValue(attrs, condition.Attribute)
			if err != nil {
				return trace.Wrap(err)
			}
			switch c := condition.Operator.(type) {
			case *workloadidentityv1pb.WorkloadIdentityCondition_Eq:
				if val != c.Eq.Value {
					continue ruleLoop
				}
			case *workloadidentityv1pb.WorkloadIdentityCondition_NotEq:
				if val == c.NotEq.Value {
					continue ruleLoop
				}
			case *workloadidentityv1pb.WorkloadIdentityCondition_In:
				if !slices.Contains(c.In.Values, val) {
					continue ruleLoop
				}
			case *workloadidentityv1pb.WorkloadIdentityCondition_NotIn:
				if slices.Contains(c.NotIn.Values, val) {
					continue ruleLoop
				}
			default:
				return trace.BadParameter("unsupported operator %T", c)
			}
		}
		return nil
	}
	// TODO: Eventually, we'll need to work support for deny rules into here.
	return trace.AccessDenied("no matching rule found")
}

func templateExtraClaims(templates *structpb.Struct, attrs *workloadidentityv1pb.Attrs) (*structpb.Struct, error) {
	// render is called recursively on list elements and struct fields.
	var render func(string, *structpb.Value, int) (*structpb.Value, error)

	const maxDepth = 10
	render = func(fieldName string, fieldValue *structpb.Value, depth int) (*structpb.Value, error) {
		if depth >= maxDepth {
			return nil, trace.BadParameter("extra_claims cannot contain more than %d levels of nesting", maxDepth)
		}

		switch value := fieldValue.GetKind().(type) {
		// Numbers, booleans, and nulls can be emitted as-is.
		case *structpb.Value_NumberValue, *structpb.Value_BoolValue, *structpb.Value_NullValue:
			return fieldValue, nil

		// We treat string values as templates.
		case *structpb.Value_StringValue:
			renderedString, err := expression.RenderTemplate(value.StringValue, &expression.Environment{Attrs: attrs})
			if err != nil {
				return nil, trace.Wrap(err, "templating claim: %s", fieldName)
			}
			return structpb.NewStringValue(renderedString), nil

		// For struct values, we call render on each of their fields.
		case *structpb.Value_StructValue:
			result := &structpb.Struct{Fields: make(map[string]*structpb.Value)}
			for structKey, structValue := range value.StructValue.GetFields() {
				keyWithPrefix := structKey
				if fieldName != "" {
					keyWithPrefix = fmt.Sprintf("%s.%s", fieldName, structKey)
				}
				v, err := render(keyWithPrefix, structValue, depth+1)
				if err != nil {
					return nil, err
				}
				result.Fields[structKey] = v
			}
			return structpb.NewStructValue(result), nil

		// For list values, we call render on each of their elements.
		case *structpb.Value_ListValue:
			result := new(structpb.ListValue)
			for idx, val := range value.ListValue.GetValues() {
				v, err := render(fmt.Sprintf("%s[%d]", fieldName, idx), val, depth+1)
				if err != nil {
					return nil, err
				}
				result.Values = append(result.Values, v)
			}
			return structpb.NewListValue(result), nil

		// At the time of writing, there are no other possible value types.
		default:
			return nil, trace.Errorf("unsupported field type: %T", value)
		}
	}

	result, err := render("", structpb.NewStructValue(templates), 0)
	if err != nil {
		return nil, err
	}
	return result.GetStructValue(), nil
}
