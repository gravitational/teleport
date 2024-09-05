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

package services

import (
	"context"

	"github.com/gravitational/trace"

	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"
)

// StaticHostUserService manages host users that should be created on SSH nodes.
type StaticHostUser interface {
	// ListStaticHostUsers lists static host users.
	ListStaticHostUsers(ctx context.Context, pageSize int, pageToken string) ([]*userprovisioningpb.StaticHostUser, string, error)
	// GetStaticHostUser returns a static host user by name.
	GetStaticHostUser(ctx context.Context, name string) (*userprovisioningpb.StaticHostUser, error)
	// CreateStaticHostUser creates a static host user.
	CreateStaticHostUser(ctx context.Context, in *userprovisioningpb.StaticHostUser) (*userprovisioningpb.StaticHostUser, error)
	// UpdateStaticHostUser updates a static host user.
	UpdateStaticHostUser(ctx context.Context, in *userprovisioningpb.StaticHostUser) (*userprovisioningpb.StaticHostUser, error)
	// UpsertStaticHostUser upserts a static host user.
	UpsertStaticHostUser(ctx context.Context, in *userprovisioningpb.StaticHostUser) (*userprovisioningpb.StaticHostUser, error)
	// DeleteStaticHostUser deletes a static host user. Note that this does not
	// remove any host users created on nodes from the resource.
	DeleteStaticHostUser(ctx context.Context, name string) error
}

// ValidateStaticHostUser checks that required parameters are set for the
// specified StaticHostUser.
func ValidateStaticHostUser(u *userprovisioningpb.StaticHostUser) error {
	// Check if required info exists.
	if u == nil {
		return trace.BadParameter("StaticHostUser is nil")
	}
	if u.Metadata == nil {
		return trace.BadParameter("Metadata is nil")
	}
	if u.Metadata.Name == "" {
		return trace.BadParameter("missing name")
	}
	if u.Spec == nil {
		return trace.BadParameter("Spec is nil")
	}

	if len(u.Spec.Matchers) == 0 {
		return trace.BadParameter("missing matchers")
	}
	for _, matcher := range u.Spec.Matchers {
		// Check if matcher can match any resources.
		if len(matcher.NodeLabels) == 0 && len(matcher.NodeLabelsExpression) == 0 {
			return trace.BadParameter("either NodeLabels or NodeLabelsExpression must be set")
		}
		for _, label := range matcher.NodeLabels {
			if label.Name == types.Wildcard && !(len(label.Values) == 1 && label.Values[0] == types.Wildcard) {
				return trace.BadParameter("selector *:<val> is not supported")
			}
		}
		if len(matcher.NodeLabelsExpression) > 0 {
			if _, err := parseLabelExpression(matcher.NodeLabelsExpression); err != nil {
				return trace.BadParameter("parsing node labels expression: %v", err)
			}
		}
	}
	return nil
}
