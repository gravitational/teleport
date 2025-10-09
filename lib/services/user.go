/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// ValidateUser validates the User and sets default values
func ValidateUser(u types.User) error {
	if err := CheckAndSetDefaults(u); err != nil {
		return trace.Wrap(err)
	}

	if localAuth := u.GetLocalAuth(); localAuth != nil {
		if err := ValidateLocalAuthSecrets(localAuth); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// ValidateUserRoles checks that all the roles in the user exist
func ValidateUserRoles(ctx context.Context, u types.User, roleGetter RoleGetter) error {
	for _, role := range u.GetRoles() {
		if _, err := roleGetter.GetRole(ctx, role); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// UsersEquals checks if the users are equal
func UsersEquals(u types.User, other types.User) bool {
	return cmp.Equal(u, other,
		ignoreProtoXXXFields(),
		cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
		cmpopts.SortSlices(func(a, b *types.MFADevice) bool {
			return a.Metadata.Name < b.Metadata.Name
		}),
	)
}

// LoginAttempt represents successful or unsuccessful attempt for user to login
type LoginAttempt struct {
	// Time is time of the attempt
	Time time.Time `json:"time"`
	// Success indicates whether attempt was successful
	Success bool `json:"bool"`
}

// Check checks parameters
func (la *LoginAttempt) Check() error {
	if la.Time.IsZero() {
		return trace.BadParameter("missing parameter time")
	}
	return nil
}

// UnmarshalUser unmarshals the User resource from JSON.
func UnmarshalUser(bytes []byte, opts ...MarshalOption) (*types.UserV2, error) {
	var h types.ResourceHeader
	err := json.Unmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch h.Version {
	case types.V2:
		var u types.UserV2
		if err := utils.FastUnmarshal(bytes, &u); err != nil {
			return nil, trace.BadParameter("%s", err)
		}

		if err := ValidateUser(&u); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.Revision != "" {
			u.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			u.SetExpiry(cfg.Expires)
		}

		return &u, nil
	}
	return nil, trace.BadParameter("user resource version %v is not supported", h.Version)
}

// MarshalUser marshals the User resource to JSON.
func MarshalUser(user types.User, opts ...MarshalOption) ([]byte, error) {
	if err := ValidateUser(user); err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch user := user.(type) {
	case *types.UserV2:
		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, user))
	default:
		return nil, trace.BadParameter("unrecognized user version %T", user)
	}
}

// UsernameForRemoteCluster returns an username that is prefixed with "remote-"
// and suffixed with cluster name with the hope that it does not match a real
// local user.
func UsernameForRemoteCluster(localUsername, localClusterName string) string {
	return fmt.Sprintf("remote-%v-%v", localUsername, localClusterName)
}
