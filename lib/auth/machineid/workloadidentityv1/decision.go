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

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
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

	// Yay - made it to the end!
	d.shouldIssue = true
	return d
}
