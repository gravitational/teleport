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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userprovisioning"
	"github.com/gravitational/teleport/lib/utils"
)

// StaticHostUserService manages host users that should be created on SSH nodes.
type StaticHostUser interface {
	// ListStaticHostUsers lists static host users.
	ListStaticHostUsers(ctx context.Context, pageSize int, pageToken string) ([]*userprovisioning.StaticHostUser, string, error)
	// GetStaticHostUser returns a static host user by name.
	GetStaticHostUser(ctx context.Context, name string) (*userprovisioning.StaticHostUser, error)
	// CreateStaticHostUser creates a static host user.
	CreateStaticHostUser(ctx context.Context, in *userprovisioning.StaticHostUser) (*userprovisioning.StaticHostUser, error)
	// UpdateStaticHostUser updates a static host user.
	UpdateStaticHostUser(ctx context.Context, in *userprovisioning.StaticHostUser) (*userprovisioning.StaticHostUser, error)
	// UpsertStaticHostUser upserts a static host user.
	UpsertStaticHostUser(ctx context.Context, in *userprovisioning.StaticHostUser) (*userprovisioning.StaticHostUser, error)
	// DeleteStaticHostUser deletes a static host user. Note that this does not
	// remove any host users created on nodes from the resource.
	DeleteStaticHostUser(ctx context.Context, name string) error
}

func MarshalStaticHostUser(hostUser *userprovisioning.StaticHostUser, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !cfg.PreserveRevision {
		copy := *hostUser
		copy.SetRevision("")
		hostUser = &copy
	}
	return utils.FastMarshal(hostUser)
}

func UnmarshalStaticHostUser(data []byte, opts ...MarshalOption) (*userprovisioning.StaticHostUser, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing static host user data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var hostUser userprovisioning.StaticHostUser
	if err := utils.FastUnmarshal(data, &hostUser); err != nil {
		return nil, trace.BadParameter(err.Error())
	}
	if cfg.Revision != "" {
		hostUser.SetRevision(cfg.Revision)
	}
	if !cfg.Expires.IsZero() {
		hostUser.SetExpiry(cfg.Expires)
	}
	return &hostUser, nil
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
func ValidateStaticHostUser(u *userprovisioning.StaticHostUser) error {
	if u == nil {
		return trace.BadParameter("StaticHostUser is nil")
	}
	if u.Metadata.Name == "" {
		return trace.BadParameter("missing name")
	}
	if u.Spec.Login == "" {
		return trace.BadParameter("missing login")
	}
	if u.Spec.NodeLabels != nil {
		for key, values := range u.Spec.NodeLabels {
			if key == types.Wildcard && !(len(values) == 1 && values[0] == types.Wildcard) {
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
