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
	"github.com/gravitational/teleport/api/types"
)

// NewCrownJewel creates a new CrownJewel object.
// It validates the object before returning it.
func NewCrownJewel(name string, spec *crownjewelv1.CrownJewelSpec) (*crownjewelv1.CrownJewel, error) {
	cj := crownjewelv1.CrownJewel_builder{
		Kind:    types.KindCrownJewel,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: name,
		}.Build(),
		Spec: spec,
	}.Build()

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
	case !jewel.HasMetadata():
		return trace.BadParameter("crown jewel metadata is nil")
	case jewel.GetMetadata().GetName() == "":
		return trace.BadParameter("crown jewel name is empty")
	case !jewel.HasSpec():
		return trace.BadParameter("crown jewel spec is nil")
	case len(jewel.GetSpec().GetTeleportMatchers()) == 0 && len(jewel.GetSpec().GetAwsMatchers()) == 0 && jewel.GetSpec().GetQuery() == "":
		return trace.BadParameter("crown jewel must have at least one matcher")
	}

	if len(jewel.GetSpec().GetTeleportMatchers()) > 0 {
		for _, matcher := range jewel.GetSpec().GetTeleportMatchers() {
			if len(matcher.GetKinds()) == 0 {
				return trace.BadParameter("teleport matcher kinds must be set")
			}

			if err := validateTeleportKinds(matcher.GetKinds()); err != nil {
				return trace.Wrap(err)
			}

			if len(matcher.GetNames()) == 0 && len(matcher.GetLabels()) == 0 {
				return trace.BadParameter("teleport matcher names or labels must be set")
			}

			for _, label := range matcher.GetLabels() {
				if label.GetName() == "" || len(label.GetValues()) == 0 {
					return trace.BadParameter("teleport matcher label name or value is empty")
				}
			}
		}
	}

	if len(jewel.GetSpec().GetAwsMatchers()) > 0 {
		for _, matcher := range jewel.GetSpec().GetAwsMatchers() {
			if len(matcher.GetTypes()) == 0 {
				return trace.BadParameter("aws matcher type must be set")
			}

			if len(matcher.GetArns()) == 0 && len(matcher.GetTags()) == 0 {
				return trace.BadParameter("aws matcher arns or tags must be set")
			}

			for _, label := range matcher.GetTags() {
				if label.GetKey() == "" || len(label.GetValues()) == 0 {
					return trace.BadParameter("aws matcher tag key or value is empty")
				}
			}
		}
	}

	return nil
}

func validateTeleportKinds(kinds []string) error {
	for _, kind := range kinds {
		switch kind {
		case types.KindUser, types.KindNode, types.KindKubeServer, types.KindApp, types.KindWindowsDesktop, types.KindDatabase:
			continue
		default:
			return trace.BadParameter("teleport matcher kind %q is not supported", kind)
		}
	}

	return nil
}
