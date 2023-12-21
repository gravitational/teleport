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
