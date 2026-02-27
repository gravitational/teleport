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
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	debugclient "github.com/gravitational/teleport/lib/client/debug"
)

// profileDoneMsg carries paths of saved profile files.
type profileDoneMsg struct {
	paths []string
}

// profileErrMsg carries a profile collection error.
type profileErrMsg struct{ err error }

// profileModel implements Tab 4: profile collection.
type profileModel struct {
	profiles    []profileEntry
	cursor      int
	seconds     int
	collecting  bool
	progress    string
	savedPaths  []string
	lastErr     error
	width       int
	height      int
}

type profileEntry struct {
	name     string
	selected bool
}

func newProfileModel() profileModel {
	names := slices.Sorted(maps.Keys(debugclient.SupportedProfiles))
	profiles := make([]profileEntry, len(names))
	for i, name := range names {
		// Default: select goroutine, heap, profile.
		selected := name == "goroutine" || name == "heap" || name == "profile"
		profiles[i] = profileEntry{name: name, selected: selected}
	}
	return profileModel{
		profiles: profiles,
		seconds:  30,
	}
}

func (m *profileModel) setSize(w, h int) {
	m.width = w
	m.height = h
}

// toggleCurrent toggles the selection of the profile under cursor.
func (m *profileModel) toggleCurrent() {
	if m.cursor >= 0 && m.cursor < len(m.profiles) {
		m.profiles[m.cursor].selected = !m.profiles[m.cursor].selected
	}
}

// moveUp moves the cursor up.
func (m *profileModel) moveUp() {
	m.cursor--
	if m.cursor < 0 {
		m.cursor = len(m.profiles) - 1
	}
}

// moveDown moves the cursor down.
func (m *profileModel) moveDown() {
	m.cursor++
	if m.cursor >= len(m.profiles) {
		m.cursor = 0
	}
}

// adjustSeconds changes the duration.
func (m *profileModel) adjustSeconds(delta int) {
	m.seconds += delta
	if m.seconds < 1 {
		m.seconds = 1
	}
	if m.seconds > 300 {
		m.seconds = 300
	}
}

// collect starts collecting selected profiles.
func (m *profileModel) collect(client *debugclient.Client, serverID string) tea.Cmd {
	if m.collecting {
		return nil
	}

	var selected []string
	for _, p := range m.profiles {
		if p.selected {
			selected = append(selected, p.name)
		}
	}
	if len(selected) == 0 {
		return nil
	}

	m.collecting = true
	m.progress = "Starting..."
	m.savedPaths = nil
	m.lastErr = nil
	seconds := m.seconds

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(seconds+30)*time.Second)
		defer cancel()

		var paths []string
		for _, name := range selected {
			data, err := client.CollectProfile(ctx, name, seconds)
			if err != nil {
				return profileErrMsg{err: fmt.Errorf("collecting %s: %w", name, err)}
			}

			filename := fmt.Sprintf("%s-%s.pb.gz", serverID, name)
			if err := os.WriteFile(filename, data, 0o600); err != nil {
				return profileErrMsg{err: fmt.Errorf("writing %s: %w", filename, err)}
			}
			paths = append(paths, filename)
		}

		return profileDoneMsg{paths: paths}
	}
}

// Update handles messages for the profile tab.
func (m *profileModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case profileDoneMsg:
		m.collecting = false
		m.progress = ""
		m.savedPaths = msg.paths
		m.lastErr = nil
	case profileErrMsg:
		m.collecting = false
		m.progress = ""
		m.lastErr = msg.err
	}
	return nil
}

// View renders the profile tab.
func (m *profileModel) View(width, height int) string {
	var b strings.Builder
	b.WriteByte('\n')

	// Duration setting
	fmt.Fprintf(&b, "  Duration: %ds (←/→ to adjust)\n\n", m.seconds)

	// Profile list with checkboxes
	b.WriteString("  Profiles:\n")
	for i, p := range m.profiles {
		check := "[ ]"
		if p.selected {
			check = "[x]"
		}
		label := fmt.Sprintf("  %s %s", check, p.name)
		if i == m.cursor {
			b.WriteString(selectedItemStyle.Render(label))
		} else {
			b.WriteString(normalItemStyle.Render(label))
		}
		b.WriteByte('\n')
	}

	b.WriteByte('\n')

	if m.collecting {
		b.WriteString(activeItemStyle.Render(fmt.Sprintf("  %s", m.progress)))
		b.WriteByte('\n')
	} else {
		b.WriteString(statusBarStyle.Render("  space: toggle | enter: collect | ←/→: adjust duration"))
		b.WriteByte('\n')
	}

	if m.lastErr != nil {
		b.WriteByte('\n')
		b.WriteString(logErrorStyle.Render(fmt.Sprintf("  Error: %v", m.lastErr)))
		b.WriteByte('\n')
	}

	if len(m.savedPaths) > 0 {
		b.WriteByte('\n')
		b.WriteString(activeItemStyle.Render("  Saved:"))
		b.WriteByte('\n')
		for _, p := range m.savedPaths {
			fmt.Fprintf(&b, "    %s\n", p)
		}
	}

	content := b.String()
	lines := strings.Count(content, "\n")
	for i := lines; i < height; i++ {
		content += "\n"
	}
	return content
}
