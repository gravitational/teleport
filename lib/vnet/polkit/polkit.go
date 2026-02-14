// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package polkit

import (
	"context"

	"github.com/godbus/dbus/v5"
	"github.com/gravitational/trace"
)

// NewSystemBusNameSubject returns a polkit subject for a system bus name.
func NewSystemBusNameSubject(name string) Subject {
	return Subject{
		Kind: SubjectKindSystemBusName,
		Details: map[string]dbus.Variant{
			SubjectDetailNameKey: dbus.MakeVariant(name),
		},
	}
}

// CheckAuthorization calls polkit's CheckAuthorization method.
func CheckAuthorization(
	ctx context.Context,
	conn *dbus.Conn,
	subject Subject,
	actionID string,
	details map[string]string,
	allowUserInteraction bool,
	cancellationID string,
) (AuthorizationResult, error) {
	var flags uint32
	if allowUserInteraction {
		flags = CheckAuthorizationFlagAllowUserInteraction
	} else {
		flags = CheckAuthorizationFlagNone
	}
	var result AuthorizationResult
	if err := conn.Object(AuthorityServiceName, dbus.ObjectPath(AuthorityObjectPath)).
		CallWithContext(ctx, CheckAuthorizationMethod, 0, subject, actionID, details, flags, cancellationID).
		Store(&result); err != nil {
		return AuthorizationResult{}, trace.Wrap(err, "checking polkit authorization")
	}
	return result, nil
}
