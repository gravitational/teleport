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

package version

import (
	"context"
	"fmt"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/automaticupgrades/constants"
)

// StaticGetter is a fake version.Getter that return a static answer. This is used
// for testing purposes.
type StaticGetter struct {
	version *semver.Version
	err     error
}

// GetVersion returns the statically defined version.
func (v StaticGetter) GetVersion(_ context.Context) (*semver.Version, error) {
	return v.version, v.err
}

// NewStaticGetter creates a StaticGetter
func NewStaticGetter(version string, err error) (Getter, error) {
	if version == constants.NoVersion {
		return StaticGetter{
			version: nil,
			err:     &NoNewVersionError{Message: fmt.Sprintf("target version set to '%s'", constants.NoVersion)},
		}, nil
	}

	if version == "" {
		// If there's no version set but a non-nil error, we are mocking an error and that's OK
		if err != nil {
			return StaticGetter{nil, err}, nil
		}
		return nil, trace.BadParameter("cannot build a static version getter from an empty version")
	}

	semVersion, err := EnsureSemver(version)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return StaticGetter{
		version: semVersion,
		err:     err,
	}, nil
}
