/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package events

import (
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"

	"github.com/gravitational/trace"
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
