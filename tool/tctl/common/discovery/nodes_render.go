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

package discovery

import (
	"fmt"
	"io"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/asciitable"
)

// renderText writes a table of discovered instances to w using asciitable.
// If instances is empty, it prints a short message and returns.
func renderText(w io.Writer, instances []instanceInfo) error {
	if len(instances) == 0 {
		_, _ = fmt.Fprintln(w, "No instances found.")
		return nil
	}

	columns := []string{"Cloud", "Account", "Region", "Instance", "Time", "Status", "Details"}
	rows := make([][]string, 0, len(instances))
	for _, inst := range instances {
		ci := inst.cloud()
		var cloudName, instance, accountID string
		if ci != nil {
			cloudName = ci.cloudName()
			instance = ci.instanceText()
			accountID = ci.cloudAccountID()
		}
		rows = append(rows, []string{
			cloudName,
			accountID,
			inst.Region,
			instance,
			inst.lastTime(),
			inst.status(),
			inst.details(),
		})
	}
	t := asciitable.MakeTableWithTruncatedColumn(columns, rows, "Details")
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
