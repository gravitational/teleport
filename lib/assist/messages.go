/*

 Copyright 2023 Gravitational, Inc.

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

package assist

import (
	"fmt"
)

// CommandExecSummary is a payload for the COMMAND_RESULT_SUMMARY message.
type CommandExecSummary struct {
	ExecutionID string `json:"execution_id"`
	Summary     string `json:"summary"`
	Command     string `json:"command"`
}

// String implements the Stringer interface and formats the message for AI
// model consumption.
func (s CommandExecSummary) String() string {
	return fmt.Sprintf("Command: `%s` executed. The command output summary is: %s", s.Command, s.Summary)
}
