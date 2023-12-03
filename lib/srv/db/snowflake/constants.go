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

package snowflake

// Borrowed from https://github.com/snowflakedb/gosnowflake/blob/e24bda449ced75324e8ce61377c88e4cea9c1efa/restful.go#L33-L42

// Snowflake Server Endpoints
const (
	loginRequestPath   = "/session/v1/login-request"
	queryRequestPath   = "/queries/v1/query-request"
	tokenRequestPath   = "/session/token-request"
	sessionRequestPath = "/session"
)

// Snowflake API has more endpoint, but for those Teleport behave as pass through proxy:
// 	/queries/v1/abort-request
//	/session/authenticator-request
//	/session/heartbeat

// teleportAuthHeaderPrefix is the prefix added to the session ID sent to the client, so we are able to distinguish between
// our own tokens and headers set by Snowflake SDK.
const teleportAuthHeaderPrefix = "Teleport:"
