/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package recordings

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"

	sessionsearchv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionsearch/v1"
)

var _ list.Item = sessionItem{}

// sessionItem wraps a SessionSummary for rendering inside a bubbles list.
type sessionItem struct {
	s *sessionsearchv1pb.SessionSummary
}

// Title returns the primary row text: "[KIND] resource_name".
func (i sessionItem) Title() string {
	name := sanitize(i.s.GetResourceName())
	if name == "" {
		name = sanitize(i.s.GetResourceId())
	}
	if name == "" {
		name = "(unknown)"
	}
	return fmt.Sprintf("[%s] %s", strings.ToUpper(sanitize(i.s.GetKind())), name)
}

// Description returns the secondary row text: "start • username • severity".
func (i sessionItem) Description() string {
	start := "-"
	if ts := i.s.GetSessionStart(); ts != nil {
		start = ts.AsTime().UTC().Format("2006-01-02 15:04 UTC")
	}
	return fmt.Sprintf("%s  •  %s  •  %s", start, sanitize(i.s.GetUsername()), formatSeverity(i.s.GetSeverity()))
}

// FilterValue is used by the list's built-in filter.
func (i sessionItem) FilterValue() string {
	return strings.Join([]string{
		i.s.GetSessionId(),
		i.s.GetUsername(),
		i.s.GetResourceName(),
		i.s.GetKind(),
	}, " ")
}
