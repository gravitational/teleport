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

const (
	AuthorityServiceName = "org.freedesktop.PolicyKit1"
	AuthorityObjectPath  = "/org/freedesktop/PolicyKit1/Authority"
	AuthorityInterface   = "org.freedesktop.PolicyKit1.Authority"

	CheckAuthorizationMethod = AuthorityInterface + ".CheckAuthorization"

	CheckAuthorizationFlagNone                 = uint32(0x00000000)
	CheckAuthorizationFlagAllowUserInteraction = uint32(0x00000001)

	SubjectKindSystemBusName = "system-bus-name"
	SubjectDetailNameKey     = "name"
)

// Subject describes the entity being authorized.
type Subject struct {
	// Kind is the subject kind, e.g. "system-bus-name".
	Kind string
	// Details are subject kind specific key/value pairs.
	Details map[string]dbus.Variant
}

// AuthorizationResult is the result of CheckAuthorization.
type AuthorizationResult struct {
	// Authorized is true if the subject is authorized for the action.
	Authorized bool
	// Challenge is true if the subject could be authorized after authentication.
	Challenge bool
	// Details contains extra result information.
	Details map[string]string
}

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
	flags := CheckAuthorizationFlagNone
	if allowUserInteraction {
		flags = CheckAuthorizationFlagAllowUserInteraction
	}
	var result AuthorizationResult
	if err := conn.Object(AuthorityServiceName, dbus.ObjectPath(AuthorityObjectPath)).
		CallWithContext(ctx, CheckAuthorizationMethod, 0, subject, actionID, details, flags, cancellationID).
		Store(&result); err != nil {
		return AuthorizationResult{}, trace.Wrap(err, "checking polkit authorization")
	}
	return result, nil
}
