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

package debuglive

import (
	"fmt"
	"strings"

	"github.com/gravitational/teleport/api/types"
)

// instance holds the display information for a cluster instance.
type instance struct {
	serverID string
	hostname string
	services []string
}

// section represents a collapsible group of instances.
type section struct {
	name      string
	role      types.SystemRole
	instances []instance
	collapsed bool
}

// pickerModel is the left-pane instance picker.
type pickerModel struct {
	sections  [3]section
	cursor    int    // cursor position in the flattened visible list
	connected int    // index of the connected instance (-1 = none)
	filter    string // current search filter
	filtering bool   // whether user is typing a filter
	width     int
	height    int
}

func newPickerModel() pickerModel {
	return pickerModel{
		sections: [3]section{
			{name: "Auth Servers", role: types.RoleAuth},
			{name: "Proxies", role: types.RoleProxy},
			{name: "Nodes", role: types.RoleNode},
		},
		connected: -1,
	}
}

// visibleItem represents a single line in the picker.
type visibleItem struct {
	sectionIdx int
	isHeader   bool
	instIdx    int // index within the section's instances slice
}

// visibleItems builds the list of visible items based on collapsed/filter state.
func (m *pickerModel) visibleItems() []visibleItem {
	var items []visibleItem
	lowerFilter := strings.ToLower(m.filter)
	for si := range m.sections {
		s := &m.sections[si]
		items = append(items, visibleItem{sectionIdx: si, isHeader: true})
		if s.collapsed {
			continue
		}
		for ii := range s.instances {
			inst := &s.instances[ii]
			if lowerFilter != "" {
				if !strings.Contains(strings.ToLower(inst.hostname), lowerFilter) &&
					!strings.Contains(strings.ToLower(inst.serverID), lowerFilter) {
					continue
				}
			}
			items = append(items, visibleItem{sectionIdx: si, instIdx: ii})
		}
	}
	return items
}

// globalIndex returns the global instance index for section si, instance ii.
// This is used to match the connected instance across refreshes.
func (m *pickerModel) globalIndex(si, ii int) int {
	idx := 0
	for s := 0; s < si; s++ {
		idx += len(m.sections[s].instances)
	}
	return idx + ii
}

// instanceAt returns the instance at the given visible item, or nil.
func (m *pickerModel) instanceAt(item visibleItem) *instance {
	if item.isHeader {
		return nil
	}
	return &m.sections[item.sectionIdx].instances[item.instIdx]
}

// selectedInstance returns the instance under the cursor, if any.
func (m *pickerModel) selectedInstance() *instance {
	items := m.visibleItems()
	if m.cursor < 0 || m.cursor >= len(items) {
		return nil
	}
	return m.instanceAt(items[m.cursor])
}

// selectedGlobalIndex returns the global index of the cursor instance, or -1.
func (m *pickerModel) selectedGlobalIndex() int {
	items := m.visibleItems()
	if m.cursor < 0 || m.cursor >= len(items) {
		return -1
	}
	item := items[m.cursor]
	if item.isHeader {
		return -1
	}
	return m.globalIndex(item.sectionIdx, item.instIdx)
}

func (m *pickerModel) moveUp() {
	items := m.visibleItems()
	if len(items) == 0 {
		return
	}
	m.cursor--
	if m.cursor < 0 {
		m.cursor = len(items) - 1
	}
}

func (m *pickerModel) moveDown() {
	items := m.visibleItems()
	if len(items) == 0 {
		return
	}
	m.cursor++
	if m.cursor >= len(items) {
		m.cursor = 0
	}
}

func (m *pickerModel) toggleCollapse() {
	items := m.visibleItems()
	if m.cursor < 0 || m.cursor >= len(items) {
		return
	}
	item := items[m.cursor]
	if item.isHeader {
		m.sections[item.sectionIdx].collapsed = !m.sections[item.sectionIdx].collapsed
	}
}

// updateInstances replaces the instance lists from a fresh GetInstances call.
func (m *pickerModel) updateInstances(instances []instance) {
	auths := make([]instance, 0)
	proxies := make([]instance, 0)
	nodes := make([]instance, 0)

	for _, inst := range instances {
		categorized := false
		for _, svc := range inst.services {
			switch types.SystemRole(svc) {
			case types.RoleAuth:
				auths = append(auths, inst)
				categorized = true
			case types.RoleProxy:
				proxies = append(proxies, inst)
				categorized = true
			case types.RoleNode:
				nodes = append(nodes, inst)
				categorized = true
			}
			if categorized {
				break
			}
		}
		if !categorized {
			nodes = append(nodes, inst)
		}
	}

	m.sections[0].instances = auths
	m.sections[1].instances = proxies
	m.sections[2].instances = nodes

	// Clamp cursor.
	items := m.visibleItems()
	if m.cursor >= len(items) {
		m.cursor = max(0, len(items)-1)
	}
}

// View renders the picker pane.
func (m *pickerModel) View(focused bool) string {
	items := m.visibleItems()
	var b strings.Builder

	// Filter bar
	if m.filtering {
		fmt.Fprintf(&b, " / %s█\n", m.filter)
	} else if m.filter != "" {
		fmt.Fprintf(&b, " / %s\n", m.filter)
	}

	contentHeight := m.height - 2 // border
	if m.filtering || m.filter != "" {
		contentHeight--
	}

	// Calculate scroll offset to keep cursor visible.
	scrollOffset := 0
	if m.cursor >= contentHeight {
		scrollOffset = m.cursor - contentHeight + 1
	}

	rendered := 0
	for i, item := range items {
		if i < scrollOffset {
			continue
		}
		if rendered >= contentHeight {
			break
		}

		if item.isHeader {
			s := &m.sections[item.sectionIdx]
			arrow := "▼"
			if s.collapsed {
				arrow = "▶"
			}
			header := fmt.Sprintf(" %s %s (%d)", arrow, s.name, len(s.instances))
			if i == m.cursor && focused {
				b.WriteString(selectedItemStyle.Render(header))
			} else {
				b.WriteString(sectionHeaderStyle.Render(header))
			}
		} else {
			inst := m.instanceAt(item)
			gIdx := m.globalIndex(item.sectionIdx, item.instIdx)
			prefix := "  "
			if gIdx == m.connected {
				prefix = " ▸"
			}
			name := inst.hostname
			if name == "" {
				name = inst.serverID[:min(12, len(inst.serverID))]
			}
			maxNameLen := m.width - 6
			if maxNameLen > 0 && len(name) > maxNameLen {
				name = name[:maxNameLen]
			}
			line := prefix + " " + name

			switch {
			case gIdx == m.connected:
				b.WriteString(activeItemStyle.Render(line))
			case i == m.cursor && focused:
				b.WriteString(selectedItemStyle.Render(line))
			default:
				b.WriteString(normalItemStyle.Render(line))
			}
		}
		b.WriteByte('\n')
		rendered++
	}

	// Pad remaining lines.
	for rendered < contentHeight {
		b.WriteByte('\n')
		rendered++
	}

	content := b.String()
	style := inactiveBorderStyle
	if focused {
		style = activeBorderStyle
	}
	return style.
		Width(m.width - 2). // account for border
		Height(contentHeight).
		Render(content)
}
