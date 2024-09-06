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
	"strings"

	"github.com/gravitational/teleport/lib/automaticupgrades/constants"
)

// StaticGetter is a fake version.Getter that return a static answer. This is used
// for testing purposes.
type StaticGetter struct {
	version string
	err     error
}

// GetVersion returns the statically defined version.
func (v StaticGetter) GetVersion(_ context.Context) (string, error) {
	return v.version, v.err
}

// NewStaticGetter creates a StaticGetter
func NewStaticGetter(version string, err error) Getter {
	if version == constants.NoVersion {
		return StaticGetter{
			version: "",
			err:     &NoNewVersionError{Message: fmt.Sprintf("target version set to '%s'", constants.NoVersion)},
		}
	}

	semVersion := version
	if semVersion != "" && !strings.HasPrefix(semVersion, "v") {
		semVersion = "v" + version
	}
	return StaticGetter{
		version: semVersion,
		err:     err,
	}
}
