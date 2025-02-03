// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package common

import (
	"fmt"
	"maps"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
)

// Labels is a common type for parsing Teleport labels from commandline args
// using kingpin
type Labels map[string]string

// IsCumulative tells kingpin it is safe to invoke [Labels.Set(string)] multiple times
func (l *Labels) IsCumulative() bool {
	return true
}

// Set implements [flag.Value] for a collection of Teleport labels
func (l *Labels) Set(value string) error {
	items, err := client.ParseLabelSpec(value)
	if err != nil {
		return trace.Wrap(err)
	}
	if *l == nil {
		*l = items
		return nil
	}
	maps.Copy(*l, items)
	return nil
}

// String implements [fmt.Stringer] for a collection of Teleport labels
func (l *Labels) String() string {
	if len(*l) == 0 {
		return ""
	}

	var buf strings.Builder
	for k, v := range *l {
		fmt.Fprintf(&buf, "%s=%s,", k, v)
	}

	// trim trailing comma and return
	result := buf.String()
	return result[:len(result)-1]
}
