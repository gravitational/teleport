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
	"strconv"

	"github.com/gravitational/trace"

	userprovisioningpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v1"
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

func isValidUidOrGid(s string) bool {
	// No uid/gid is OK.
	if s == "" {
		return true
	}
	// If uid/gid is present, it must be an integer (uid/gid are strings instead
	// of ints to match user traits).
	_, err := strconv.Atoi(s)
	return err == nil
}

// ValidateStaticHostUser checks that required parameters are set for the
// specified StaticHostUser.
func ValidateStaticHostUser(u *userprovisioningpb.StaticHostUser) error {
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
	if u.Spec.Login == "" {
		return trace.BadParameter("missing login")
	}
	if u.Spec.NodeLabels != nil {
		for key, value := range u.Spec.NodeLabels.Values {
			if key == types.Wildcard && !(len(value.Values) == 1 && value.Values[0] == types.Wildcard) {
				return trace.BadParameter("selector *:<val> is not supported")
			}
		}
	}
	if len(u.Spec.NodeLabelsExpression) > 0 {
		if _, err := parseLabelExpression(u.Spec.NodeLabelsExpression); err != nil {
			return trace.BadParameter("parsing node labels expression: %v", err)
		}
	}
	if !isValidUidOrGid(u.Spec.Uid) {
		return trace.BadParameter("invalid uid: %q", u.Spec.Uid)
	}
	if !isValidUidOrGid(u.Spec.Gid) {
		return trace.BadParameter("invalid gid: %q", u.Spec.Gid)
	}
	return nil
}
