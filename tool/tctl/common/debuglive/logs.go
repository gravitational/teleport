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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	debugclient "github.com/gravitational/teleport/lib/client/debug"
)

const maxLogLines = 1000

var (
	logLevels         = []string{"", "TRACE", "DEBUG", "INFO", "WARN", "ERROR"}
	editableLogLevels = []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR"}
)

// logEntry holds a parsed log line.
type logEntry struct {
	raw       string
	level     string
	timestamp string
	message   string
	component string
}

// logEntriesMsg carries new log entries from the streaming goroutine.
type logEntriesMsg struct {
	entries []logEntry
	gen     uint64 // connection generation to detect stale messages
}

// logStreamErrMsg signals a stream error.
type logStreamErrMsg struct {
	err error
	gen uint64
}

// readinessMsg carries a readiness result from a fetch.
type readinessMsg struct {
	readiness debugclient.Readiness
	logLevel  string
}

// readinessErrMsg carries a readiness fetch error.
type readinessErrMsg struct{ err error }

// logLevelSetMsg signals that the log level was successfully changed.
type logLevelSetMsg struct{ result string }

// logsModel implements the combined Logs + Status tab: live log stream with
// a status header showing readiness, PID, and log level control.
type logsModel struct {
	// Log stream state.
	lines     []logEntry
	levelIdx  int    // index into logLevels for minimum level filter
	component string // component filter
	paused    bool
	scrollPos int // viewport scroll position (0 = bottom/latest)

	// Status state (merged from the old status tab).
	readiness   *debugclient.Readiness
	logLevel    string // current server log level
	logLevelIdx int    // index into editableLogLevels for the selector

	width  int
	height int
}

func newLogsModel() logsModel {
	return logsModel{
		levelIdx: 3, // default to INFO
	}
}

func (m *logsModel) setSize(w, h int) {
	m.width = w
	m.height = h
}

func parseLogLine(line string) logEntry {
	entry := logEntry{raw: line}
	var obj map[string]any
	if json.Unmarshal([]byte(line), &obj) == nil {
		if lvl, ok := obj["level"].(string); ok {
			entry.level = strings.ToUpper(lvl)
		}
		if ts, ok := obj["timestamp"].(string); ok {
			if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
				entry.timestamp = t.Format("15:04:05")
			} else {
				entry.timestamp = ts
			}
		} else if ts, ok := obj["time"].(string); ok {
			if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
				entry.timestamp = t.Format("15:04:05")
			} else {
				entry.timestamp = ts
			}
		}
		if msg, ok := obj["message"].(string); ok {
			entry.message = msg
		} else if msg, ok := obj["msg"].(string); ok {
			entry.message = msg
		}
		if comp, ok := obj["component"].(string); ok {
			entry.component = comp
		}
	}
	return entry
}

// fetchStatus creates a command to fetch readiness and log level.
func (m *logsModel) fetchStatus(client *debugclient.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		readiness, err := client.GetReadiness(ctx)
		if err != nil {
			return readinessErrMsg{err: err}
		}

		logLevel, err := client.GetLogLevel(ctx)
		if err != nil {
			return readinessErrMsg{err: err}
		}

		return readinessMsg{
			readiness: readiness,
			logLevel:  strings.TrimSpace(logLevel),
		}
	}
}

// setLogLevel creates a command to set the log level on the server.
func (m *logsModel) setLogLevel(client *debugclient.Client, level string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result, err := client.SetLogLevel(ctx, level)
		if err != nil {
			return readinessErrMsg{err: err}
		}
		return logLevelSetMsg{result: strings.TrimSpace(result)}
	}
}

// Update handles messages for the logs tab. The currentGen parameter
// is used to discard stale messages from previous connections.
func (m *logsModel) Update(msg tea.Msg, currentGen uint64) tea.Cmd {
	switch msg := msg.(type) {
	case logEntriesMsg:
		if msg.gen != currentGen {
			return nil // stale message from previous connection
		}
		if !m.paused {
			m.lines = append(m.lines, msg.entries...)
			if len(m.lines) > maxLogLines {
				m.lines = m.lines[len(m.lines)-maxLogLines:]
			}
		}
	case logStreamErrMsg:
		if msg.gen != currentGen {
			return nil
		}
		m.lines = append(m.lines, logEntry{
			raw:   fmt.Sprintf("Stream error: %v", msg.err),
			level: "ERROR",
		})
	case readinessMsg:
		m.readiness = &msg.readiness
		m.logLevel = msg.logLevel
		for i, lvl := range editableLogLevels {
			if strings.EqualFold(lvl, m.logLevel) {
				m.logLevelIdx = i
				break
			}
		}
	case logLevelSetMsg:
		m.logLevel = msg.result
	}
	return nil
}

// cycleLevelFilter advances the minimum log level filter.
func (m *logsModel) cycleLevelFilter() {
	m.levelIdx = (m.levelIdx + 1) % len(logLevels)
}

// togglePause toggles the pause state.
func (m *logsModel) togglePause() {
	m.paused = !m.paused
}

// cycleLogLevel moves the server log level selector left (-1) or right (+1).
func (m *logsModel) cycleLogLevel(dir int) {
	m.logLevelIdx += dir
	if m.logLevelIdx < 0 {
		m.logLevelIdx = len(editableLogLevels) - 1
	}
	if m.logLevelIdx >= len(editableLogLevels) {
		m.logLevelIdx = 0
	}
}

// selectedLogLevel returns the currently selected (not yet applied) level.
func (m *logsModel) selectedLogLevel() string {
	return editableLogLevels[m.logLevelIdx]
}

// scrollUp scrolls the viewport up.
func (m *logsModel) scrollUp() {
	m.scrollPos++
	maxScroll := max(0, len(m.lines)-m.height+4) // header takes extra lines
	if m.scrollPos > maxScroll {
		m.scrollPos = maxScroll
	}
	if m.scrollPos > 0 {
		m.paused = true
	}
}

// scrollDown scrolls the viewport down.
func (m *logsModel) scrollDown() {
	m.scrollPos--
	if m.scrollPos <= 0 {
		m.scrollPos = 0
		m.paused = false
	}
}

// View renders the combined logs + status viewport.
func (m *logsModel) View(width, height int) string {
	var b strings.Builder

	// Status header line: readiness + PID + log level selector.
	b.WriteString(m.statusHeader(width))
	b.WriteByte('\n')

	// Stream filter line.
	levelLabel := logLevels[m.levelIdx]
	if levelLabel == "" {
		levelLabel = "ALL"
	}
	stream := fmt.Sprintf(" Stream: %s", levelLabel)
	if m.component != "" {
		stream += fmt.Sprintf(" | Component: %s", m.component)
	}
	if m.paused {
		stream += " | PAUSED"
	}
	stream += fmt.Sprintf(" | %d lines", len(m.lines))
	b.WriteString(statusBarStyle.Render(stream))
	b.WriteByte('\n')

	headerLines := 2
	viewHeight := height - headerLines

	// Filter lines by component if set.
	var filtered []logEntry
	for i := range m.lines {
		entry := &m.lines[i]
		if m.component != "" && entry.component != "" {
			if !strings.Contains(strings.ToLower(entry.component), strings.ToLower(m.component)) {
				continue
			}
		}
		filtered = append(filtered, *entry)
	}

	// Calculate visible window.
	start := max(0, len(filtered)-viewHeight-m.scrollPos)
	end := min(max(0, len(filtered)-m.scrollPos), len(filtered))
	if start > end {
		start = end
	}

	rendered := 0
	for i := start; i < end && rendered < viewHeight; i++ {
		entry := &filtered[i]
		line := m.formatLogLine(entry, width)
		b.WriteString(line)
		b.WriteByte('\n')
		rendered++
	}

	for rendered < viewHeight {
		b.WriteByte('\n')
		rendered++
	}

	return b.String()
}

// statusHeader renders the compact status bar with readiness, PID, and log level selector.
func (m *logsModel) statusHeader(width int) string {
	if m.readiness == nil {
		return statusBarStyle.Render(" Loading status...")
	}

	var parts []string

	// Readiness indicator.
	if m.readiness.Ready {
		parts = append(parts, activeItemStyle.Render("READY"))
	} else {
		parts = append(parts, logErrorStyle.Render("NOT READY"))
	}

	// PID.
	parts = append(parts, statusBarStyle.Render(fmt.Sprintf("PID: %d", m.readiness.PID)))

	// Log level selector.
	var lvlParts []string
	for i, lvl := range editableLogLevels {
		if i == m.logLevelIdx {
			lvlParts = append(lvlParts, selectedItemStyle.Render(fmt.Sprintf("[%s]", lvl)))
		} else {
			lvlParts = append(lvlParts, statusBarStyle.Render(fmt.Sprintf(" %s ", lvl)))
		}
	}
	parts = append(parts, "Level: "+strings.Join(lvlParts, ""))

	line := " " + strings.Join(parts, "  ")
	if len(line) > width {
		line = line[:width]
	}
	return line
}

func (m *logsModel) formatLogLine(entry *logEntry, width int) string {
	if entry.message == "" {
		line := entry.raw
		if len(line) > width {
			line = line[:width]
		}
		return m.colorByLevel(line, entry.level)
	}

	var parts []string
	if entry.timestamp != "" {
		parts = append(parts, entry.timestamp)
	}
	if entry.level != "" {
		parts = append(parts, lipgloss.NewStyle().Width(5).Render(entry.level))
	}
	if entry.component != "" {
		parts = append(parts, fmt.Sprintf("[%s]", entry.component))
	}
	parts = append(parts, entry.message)
	line := strings.Join(parts, " ")
	if len(line) > width {
		line = line[:width]
	}
	return m.colorByLevel(line, entry.level)
}

func (m *logsModel) colorByLevel(text, level string) string {
	switch level {
	case "ERROR":
		return logErrorStyle.Render(text)
	case "WARN", "WARNING":
		return logWarnStyle.Render(text)
	case "INFO":
		return logInfoStyle.Render(text)
	case "DEBUG", "TRACE":
		return logDebugStyle.Render(text)
	default:
		return text
	}
}
