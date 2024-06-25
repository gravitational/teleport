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
	"errors"
	"regexp"
	"strconv"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

var (
	// pattern precompiled regular expression to parse teleport version.
	pattern = regexp.MustCompile("(?P<major>\\d+)\\.(?P<minor>\\d+)\\.(?P<patch>\\d+)(-(?P<suffix>\\w+))?")

	// errMajorVersionUpgrade default error for restricting upgrade major version
	errMajorVersionUpgrade = errors.New("upgrade major version must be iterative. See: https://goteleport.com/docs/upgrading/overview/#component-compatibility")
)

// Version service manages version verification in init process.
type Version interface {
	GetTeleportVersion(context.Context) (types.Version, error)
}

// VersionInternal is interface to persist information about version
// in backend storage.
type VersionInternal interface {
	Version

	// UpsertTeleportVersion creates or updates current teleport version.
	UpsertTeleportVersion(ctx context.Context, version types.Version) (types.Version, error)
}

// ValidateMajorVersion validates that the major version persistent in the backend
// meets our upgrade compatibility guide.
func ValidateMajorVersion(ctx context.Context, currentVersion string, service VersionInternal) error {
	v, err := service.GetTeleportVersion(ctx)
	if trace.IsNotFound(err) {
		return setVersion(ctx, currentVersion, service)
	} else if err != nil {
		return trace.Wrap(err)
	}

	currenMajor, err := getMajorVersion(currentVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	persistentMajor, err := getMajorVersion(v.GetTeleportVersion())
	if err != nil {
		return trace.Wrap(err)
	}
	if currenMajor-persistentMajor > 1 {
		return trace.Wrap(errMajorVersionUpgrade, "Backend version %s is going to update to %s",
			v.GetTeleportVersion(), currentVersion)
	}
	return setVersion(ctx, currentVersion, service)
}

// UnmarshalVersion unmarshalls the Version resource from JSON.
func UnmarshalVersion(bytes []byte, opts ...MarshalOption) (types.Version, error) {
	if len(bytes) == 0 {
		return nil, trace.BadParameter("missing resource data")
	}

	var v types.VersionV3
	err := utils.FastUnmarshal(bytes, &v)
	if err != nil {
		return nil, trace.BadParameter(err.Error())
	}

	if v.Version != types.V1 {
		return nil, trace.BadParameter("unsupported version %v, expected version %v", v.Version, types.V1)
	}

	if err := v.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &v, nil
}

// MarshalVersion marshals the Version resource to JSON.
func MarshalVersion(version types.Version, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch v := version.(type) {
	case *types.VersionV3:
		if err := v.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		if !cfg.PreserveRevision {
			copy := *v
			copy.SetRevision("")
			v = &copy
		}
		return utils.FastMarshal(version)
	default:
		return nil, trace.BadParameter("unrecognized version %T", version)
	}
}

// setVersion persists specified version.
func setVersion(ctx context.Context, currentVersion string, service VersionInternal) error {
	v, err := types.NewVersion(currentVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := service.UpsertTeleportVersion(ctx, v); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// getMajorVersion parses string to fetch major version number.
func getMajorVersion(version string) (int, error) {
	matches := pattern.FindStringSubmatch(version)
	if matches == nil {
		return 0, trace.BadParameter("cannot parse version: %q", version)
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, trace.Wrap(err, "invalid major version number: %s", matches[1])
	}

	return major, nil
}
