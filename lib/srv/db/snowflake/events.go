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

import (
	"fmt"

	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/srv/db/common"
)

// makeQueryEvent returns audit event for Snowflake query message which
// is sent by the client when executing a database query.
func makeQueryEvent(session *common.Session, query *queryRequest) events.AuditEvent {
	return &events.SnowflakeQuery{
		Metadata: common.MakeEventMetadata(session,
			libevents.DatabaseSessionSnowflakeQuery,
			libevents.DatabaseSessionQueryCode), //TODO(jakule): Should this be a custom event code or reusing the Session query is fine?
		UserMetadata:     common.MakeUserMetadata(session),
		SessionMetadata:  common.MakeSessionMetadata(session),
		DatabaseMetadata: common.MakeDatabaseMetadata(session),
		Query:            query.SQLText,
		Parameters:       queryParametersToProto(query.Parameters),
		Bindings:         queryBindingsToProto(query.Bindings),
		BindStage:        query.BindStage,
	}
}

func queryParametersToProto(parameters map[string]interface{}) map[string]string {
	params := make(map[string]string)

	for k, v := range parameters {
		params[k] = fmt.Sprintf("%v", v)
	}

	return params
}

func queryBindingsToProto(bindings map[string]execBindParameter) map[string]*events.SnowflakeQuery_BindParameter {
	values := make(map[string]*events.SnowflakeQuery_BindParameter)

	for k, v := range bindings {
		values[k] = &events.SnowflakeQuery_BindParameter{
			Type:  v.Type,
			Value: fmt.Sprintf("%v", v.Value),
		}
	}

	return values
}
