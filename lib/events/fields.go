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

package events

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
)

// ValidateServerMetadata checks that event server ID of the event
// if present, matches the passed server ID and namespace has proper syntax
func ValidateServerMetadata(event apievents.AuditEvent, serverID string, isProxy bool) error {
	getter, ok := event.(ServerMetadataGetter)
	if !ok {
		return nil
	}
	switch {
	case getter.GetForwardedBy() == "" && getter.GetServerID() == serverID:
	case isProxy && getter.GetForwardedBy() == serverID:
	default:
		return trace.BadParameter("server %q can't emit event with server ID %q", serverID, getter.GetServerID())
	}
	if ns := getter.GetServerNamespace(); ns != "" && !types.IsValidNamespace(ns) {
		return trace.BadParameter("invalid namespace %q", ns)
	}
	return nil
}
