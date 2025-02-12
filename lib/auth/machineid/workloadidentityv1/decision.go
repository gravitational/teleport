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
	"regexp"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/lib/utils"
)

type decision struct {
	templatedWorkloadIdentity *workloadidentityv1pb.WorkloadIdentity
	shouldIssue               bool
	reason                    error
}

func decide(
	ctx context.Context,
	wi *workloadidentityv1pb.WorkloadIdentity,
	attrs *workloadidentityv1pb.Attrs,
) *decision {
	d := &decision{
		templatedWorkloadIdentity: proto.Clone(wi).(*workloadidentityv1pb.WorkloadIdentity),
	}

	// First, evaluate the rules.
	if err := evaluateRules(wi, attrs); err != nil {
		d.reason = trace.Wrap(err, "attributes did not pass rule evaluation")
		return d
	}

	// Now we can cook up some templating...
	templated, err := templateString(wi.GetSpec().GetSpiffe().GetId(), attrs)
	if err != nil {
		d.reason = trace.Wrap(err, "templating spec.spiffe.id")
		return d
	}
	d.templatedWorkloadIdentity.Spec.Spiffe.Id = templated

	templated, err = templateString(wi.GetSpec().GetSpiffe().GetHint(), attrs)
	if err != nil {
		d.reason = trace.Wrap(err, "templating spec.spiffe.hint")
		return d
	}
	d.templatedWorkloadIdentity.Spec.Spiffe.Hint = templated

	for i, san := range wi.GetSpec().GetSpiffe().GetX509().GetDnsSans() {
		templated, err = templateString(san, attrs)
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

		templated, err = templateString(st.CommonName, attrs)
		if err != nil {
			d.reason = trace.Wrap(
				err,
				"templating spec.spiffe.x509.subject_template.common_name",
			)
			return d
		}
		dst.CommonName = templated

		templated, err = templateString(st.Organization, attrs)
		if err != nil {
			d.reason = trace.Wrap(
				err,
				"templating spec.spiffe.x509.subject_template.organization",
			)
			return d
		}
		dst.Organization = templated

		templated, err = templateString(st.OrganizationalUnit, attrs)
		if err != nil {
			d.reason = trace.Wrap(
				err,
				"templating spec.spiffe.x509.subject_template.organizational_unit",
			)
			return d
		}
		dst.OrganizationalUnit = templated
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
// TODO(noah): This function will be replaced by the Teleport predicate language
// in a coming PR.
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

// templateString takes a given input string and replaces any values within
// {{ }} with values from the attribute set.
//
// If the specified value is not found in the attribute set, an error is
// returned.
//
// TODO(noah): In a coming PR, this will be replaced by evaluating the values
// within the handlebars as expressions.
func templateString(in string, attrs *workloadidentityv1pb.Attrs) (string, error) {
	if len(in) == 0 {
		return in, nil
	}

	re := regexp.MustCompile(`\{\{([^{}]+?)\}\}`)
	matches := re.FindAllStringSubmatch(in, -1)

	for _, match := range matches {
		attrKey := strings.TrimSpace(match[1])
		value, err := getFieldStringValue(attrs, attrKey)
		if err != nil {
			return "", trace.Wrap(err, "fetching attribute value for %q", attrKey)
		}
		// We want to have an implicit rule here that if an attribute is
		// included in the template, but is not set, we should refuse to issue
		// the credential.
		if value == "" {
			return "", trace.NotFound("attribute %q unset", attrKey)
		}
		in = strings.Replace(in, match[0], value, 1)
	}

	return in, nil
}

func evaluateRules(
	wi *workloadidentityv1pb.WorkloadIdentity,
	attrs *workloadidentityv1pb.Attrs,
) error {
	if len(wi.GetSpec().GetRules().GetAllow()) == 0 {
		return nil
	}
ruleLoop:
	for _, rule := range wi.GetSpec().GetRules().GetAllow() {
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
