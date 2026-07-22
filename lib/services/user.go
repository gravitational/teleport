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
	"strings"
	"time"

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

// UsernameForClusterConfig is a configuration struct for UsernameForCluster.
type UsernameForClusterConfig struct {
	// User is the username.
	User string
	// OriginClusterName is the cluster name where the user is authenticated.
	OriginClusterName string
	// LocalClusterName is the local cluster name.
	LocalClusterName string
}

// UsernameForCluster returns an username that is prefixed with "remote-"
// and suffixed with cluster name if the user is from a remote cluster,
// otherwise returns the local username.
func UsernameForCluster(cfg UsernameForClusterConfig) string {
	// originClusterName == "" is a special case for backward compatibility
	// with older clients that do not send origin cluster name.
	// In this case we assume the user is local.
	if cfg.OriginClusterName == cfg.LocalClusterName || cfg.OriginClusterName == "" {
		return cfg.User
	}
	return UsernameForRemoteCluster(cfg.User, cfg.OriginClusterName)
}

// ResolveUserDisplays resolves usernames to display values keyed by username,
// reading each unique name once through getter via types.User.GetDisplay.
//
// Missing users are absent from the result. A user with no distinct display is
// present with a zero-value types.UserDisplay. Blank usernames are skipped, and
// any non-NotFound error aborts with no partial map.
func ResolveUserDisplays(ctx context.Context, getter UserGetter, usernames []string) (map[string]types.UserDisplay, error) {
	displays := make(map[string]types.UserDisplay)
	seen := make(map[string]struct{})

	for _, username := range usernames {
		if strings.TrimSpace(username) == "" {
			continue // skipping whitespace-only username
		}

		if _, ok := seen[username]; ok {
			continue // skipping duplicate username
		}
		seen[username] = struct{}{}

		user, err := getter.GetUser(ctx, username, false)
		if trace.IsNotFound(err) {
			continue // skipping missing user
		}
		if err != nil {
			return nil, trace.Wrap(err, "resolving display for user %q", username)
		}
		displays[username] = user.GetDisplay()
	}

	return displays, nil
}
