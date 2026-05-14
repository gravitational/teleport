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

package eventschema

// This list is manually curated to contain events that are relevant to the user
// in a security report context.
var eventTypes = []string{
	"access_list.create",
	"access_list.delete",
	"access_list.member.create",
	"access_list.member.delete",
	"access_list.member.update",
	"access_list.review",
	"access_list.update",
	"access_request.create",
	"access_request.review",
	"auth",
	"bot.join",
	"cert.create",
	"db.session.query",
	"db.session.query.failed",
	"db.session.start",
	"device.authenticate",
	"device.enroll",
	"exec",
	"instance.join",
	"join_token.create",
	"kube.request",
	"lock.created",
	"lock.deleted",
	"recovery_code.used",
	"reset_password_token.create",
	"role.created",
	"role.deleted",
	"role.updated",
	"saml.idp.auth",
	"session.command",
	"session.join",
	"session.rejected",
	"session.start",
	"user.create",
	"user.login",
	"user.password_change",
	"windows.desktop.session.end",
	"windows.desktop.session.start",
}
