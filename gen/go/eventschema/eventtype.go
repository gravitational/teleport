// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eventschema

// This list is manually curated to contain events that are relevant to the user
// in a security report context.
var eventTypes = []string{
	"access_request.create",
	"access_request.review",
	"auth",
	"bot.join",
	"bot_token.create",
	"cert.create",
	"db.session.query",
	"db.session.query.failed",
	"db.session.start",
	"device.authenticate",
	"device.enroll",
	"instance.join",
	"join_token.create",
	"lock.created",
	"lock.deleted",
	"recovery_code.used",
	"reset_password_token.create",
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
