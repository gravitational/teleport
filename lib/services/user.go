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
	"slices"
	"strconv"
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

// ValidateRoleChangesWithMinimums checks that role changes involving removing a role with minimum requirements is valid
// todo mberg re-write
func ValidateRoleChangesWithMinimums(ctx context.Context, user, req types.User, roleGetter RoleGetter) error {
	if user.GetUserType() != types.UserTypeLocal {
		// we only validate minimums for local users
		return nil
	}

	userRoles := user.GetRoles()
	requestRoles := req.GetRoles()

	if slices.Equal(userRoles, requestRoles) {
		// no change in roles, nothing to verify - continue
		return nil
	}

	// Only check minimum assignment if user both has the role, and the request removes the role
	for _, role := range userRoles {
		r, err := roleGetter.GetRole(ctx, role)
		if err != nil {
			return trace.Wrap(err)
		}

		labels := r.GetAllLabels()
		label, ok := labels[types.TeleportMinimumAssignment]
		if ok {
			// find role in request
			found := false
			for _, requestRole := range requestRoles {
				rr, err := roleGetter.GetRole(ctx, requestRole)
				if err != nil {
					return trace.Wrap(err)
				}
				if rr == r {
					found = true
				}
			}

			// previously assigned role with minimum is not in the request and will be removed
			// check if the count is valid
			if !found {
				// convert the label string to the minimum value
				minimum, err := strconv.ParseInt(label, 10, 64)
				if err != nil {
					return trace.Wrap(err)
				}

				// check to see if the role can be removed without breaking the minimum assignment requirement
				ok, err := roleGetter.VerifyMinimumRoleRemoval(ctx, r, minimum)
				if err != nil {
					return trace.Wrap(err)
				}
				// check if we decrease this count by 1, will the number of assigned drop below the minimum value
				if !ok {
					return trace.BadParameter("Unable to remove role %v from user %v as this violates the minimum role assignment", r.GetName(), user.GetName())
				}
			}
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
			return nil, trace.BadParameter(err.Error())
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
