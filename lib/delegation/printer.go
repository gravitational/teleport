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

package delegation

import (
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/trace"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
)

// Printer prints a human-friendly representation of the delegation chain.
type Printer struct {
	w io.Writer
	c PrinterConfig
}

// PrinterConfig provides optional configuration for a Printer.
type PrinterConfig struct {
	// IndentSpaces controls the number of spaces that will be prepended to each
	// line of the printer's output for indentation.
	IndentSpaces int

	// IndentFirstLine determines whether the first line of the printer's output
	// will be indented according to IndentSpaces. Setting it to false allows you
	// to control the indentation yourself if you want to print after a leading
	// label (e.g. "Delegation:    <start of output>").
	IndentFirstLine bool
}

// NewPrinter creates a Printer with the given writer.
func NewPrinter(writer io.Writer, cfg PrinterConfig) *Printer {
	return &Printer{
		w: writer,
		c: cfg,
	}
}

// Print the given delegation chain.
func (p *Printer) Print(delegation *delegationv1.Delegation) error {
	var b strings.Builder
	for i, l := range delegation.GetChain() {
		if i == 0 {
			if p.c.IndentFirstLine {
				b.WriteString(strings.Repeat(" ", p.c.IndentSpaces))
			}
		} else {
			b.WriteString(strings.Repeat(" ", p.c.IndentSpaces+2+(i-1)*4))
			b.WriteString("└── ")
		}
		b.WriteString(p.formatChainLink(l))
		b.WriteRune('\n')
	}

	_, err := io.WriteString(p.w, b.String())
	return trace.Wrap(err)
}

func (p *Printer) formatChainLink(link *delegationv1.DelegationChainLink) string {
	switch link.WhichIdentity() {
	case delegationv1.DelegationChainLink_User_case:
		return fmt.Sprintf("user (%s)", link.GetUser().GetName())
	case delegationv1.DelegationChainLink_Bot_case:
		if beam := link.GetBot().GetBeam(); beam != nil {
			return fmt.Sprintf("beam (%s)", beam.GetAlias())
		}
		return fmt.Sprintf("bot (%s)", link.GetBot().GetName())
	default:
		return "unknown"
	}
}
