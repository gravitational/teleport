// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package authz

// The items exported here exist solely to prevent import cycles and facilitate
// preexisting tests in lib/authz which relied on unexported items. All new
// tests in lib/authz should exist in the authz_test package and not rely on
// internal state.

func DisableContextDeviceRoleMode(ctx *Context) {
	ctx.disableDeviceRoleMode = true
}

func ContextDeviceRoleMode(ctx *Context) bool {
	return ctx.disableDeviceRoleMode
}
