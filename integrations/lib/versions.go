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

package lib

import (
	"github.com/gravitational/trace"
	"github.com/hashicorp/go-version"

	"github.com/gravitational/teleport/api/client/proto"
)

// AssertServerVersion returns an error if server version in ping response is
// less than minimum required version.
func AssertServerVersion(pong proto.PingResponse, minVersion string) error {
	actual, err := version.NewVersion(pong.ServerVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	required, err := version.NewVersion(minVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	if actual.LessThan(required) {
		return trace.Errorf("server version %s is less than %s", pong.ServerVersion, minVersion)
	}
	return nil
}
