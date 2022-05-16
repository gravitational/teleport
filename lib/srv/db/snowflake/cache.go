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

// tokenCache is a simple in memory map between Teleport WebSession ID and Snowflake tokens.
type tokenCache struct {
	sessionToken      string
	teleportSessionID string

	masterToken      string
	teleportMasterID string
}

func (t *tokenCache) getSessionToken(sessionID string) string {
	if t.teleportSessionID == sessionID {
		return t.sessionToken
	}
	return ""
}

func (t *tokenCache) setSessionToken(sessionID, snowflakeToken string) {
	t.sessionToken = snowflakeToken
	t.teleportSessionID = sessionID
}

func (t *tokenCache) getMasterToken(masterID string) string {
	if t.teleportMasterID == masterID {
		return t.masterToken
	}
	return ""
}

func (t *tokenCache) setMasterToken(masterID, snowflakeMasterToken string) {
	t.sessionToken = snowflakeMasterToken
	t.teleportSessionID = masterID
}
