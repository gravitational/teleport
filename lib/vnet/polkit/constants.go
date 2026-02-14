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
