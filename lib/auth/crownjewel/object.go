/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package crownjewel

import (
	"github.com/gravitational/trace"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
)

// NewCrownJewel creates a new CrownJewel object.
// It validates the object before returning it.
func NewCrownJewel(name string, spec *crownjewelv1.CrownJewelSpec) (*crownjewelv1.CrownJewel, error) {
	cj := &crownjewelv1.CrownJewel{
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: spec,
	}

	if err := ValidateCrownJewel(cj); err != nil {
		return nil, trace.Wrap(err)
	}

	return cj, nil
}

// ValidateCrownJewel validates the CrownJewel object without modifying it.
// Required fields:
//   - Metadata.Name
//   - Spec.TeleportMatchers or Spec.AwsMatchers
//   - Matcher.Name or Matcher.Labels
func ValidateCrownJewel(jewel *crownjewelv1.CrownJewel) error {
	switch {
	case jewel == nil:
		return trace.BadParameter("crown jewel is nil")
	case jewel.Metadata == nil:
		return trace.BadParameter("crown jewel metadata is nil")
	case jewel.Metadata.Name == "":
		return trace.BadParameter("crown jewel name is empty")
	case jewel.Spec == nil:
		return trace.BadParameter("crown jewel spec is nil")
	case len(jewel.Spec.TeleportMatchers) == 0 && len(jewel.Spec.AwsMatchers) == 0:
		return trace.BadParameter("crown jewel must have at least one matcher")
	}

	if len(jewel.Spec.TeleportMatchers) > 0 {
		for _, matcher := range jewel.Spec.TeleportMatchers {
			if len(matcher.GetKinds()) == 0 {
				return trace.BadParameter("teleport matcher kinds must be set")
			}

			if matcher.Name == "" && len(matcher.GetLabels()) == 0 {
				return trace.BadParameter("teleport matcher name or labels must be set")
			}

			for _, label := range matcher.GetLabels() {
				if label.Name == "" || len(label.Values) == 0 {
					return trace.BadParameter("teleport matcher label name or value is empty")
				}
			}
		}
	}

	if len(jewel.Spec.AwsMatchers) > 0 {
		for _, matcher := range jewel.Spec.AwsMatchers {
			if len(matcher.GetTypes()) == 0 {
				return trace.BadParameter("aws matcher type must be set")
			}

			if matcher.GetArn() == "" && len(matcher.GetTags()) == 0 {
				return trace.BadParameter("aws matcher arn or tags must be set")
			}

			for _, label := range matcher.GetTags() {
				if label.Key == "" || len(label.Values) == 0 {
					return trace.BadParameter("aws matcher tag key or value is empty")
				}
			}
		}
	}

	return nil
}
