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
	"encoding/json"
	"fmt"
	"io"

	"github.com/gravitational/teleport/lib/asciitable"
)

// renderText writes a table of discovered instances to w using asciitable.
// If instances is empty, it prints a short message and returns.
func renderText(w io.Writer, instances []instanceInfo) error {
	if len(instances) == 0 {
		_, err := fmt.Fprintln(w, "No instances found.")
		return err
	}

	t := asciitable.MakeTable([]string{"Instance ID", "Region", "Account", "Online", "Time", "Result", "SSM Output"})
	for _, inst := range instances {
		online := "no"
		if inst.IsOnline {
			online = "yes"
		}
		t.AddRow([]string{
			inst.InstanceID,
			inst.Region,
			inst.AccountID,
			online,
			inst.lastTime(),
			inst.result(),
			inst.ssmOutput(),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return err
}

// renderJSON encodes instances as JSON with 2-space indentation.
// A nil slice is encoded as an empty array [] rather than null.
func renderJSON(w io.Writer, instances []instanceInfo) error {
	if instances == nil {
		instances = []instanceInfo{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(instances)
}
