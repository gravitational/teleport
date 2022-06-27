/*

 Copyright 2022 Gravitational, Inc.

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
