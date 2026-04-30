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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	sessionsearchv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionsearch/v1"
	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

type summaryLoadedMsg struct {
	summary *summarizerv1pb.Summary
	err     error
}

type closeSummaryMsg struct{}

type summaryPopupModel struct {
	session       *sessionsearchv1pb.SessionSummary
	summaryGetter SummaryGetter
	ctx           context.Context
	cancel        context.CancelFunc

	summary       *summarizerv1pb.Summary
	summaryStatus string // "loading" | "loaded" | "error" | "unavailable"

	viewport viewport.Model
	keys     summaryKeyMap
	palette  palette
	width    int
	height   int
}

func newSummaryPopupModel(
	ctx context.Context,
	s *sessionsearchv1pb.SessionSummary,
	summaryGetter SummaryGetter,
	p palette,
	width, height int,
) *summaryPopupModel {
	ctx, cancel := context.WithCancel(ctx)

	m := &summaryPopupModel{
		session:       s,
		summaryGetter: summaryGetter,
		ctx:           ctx,
		cancel:        cancel,
		summaryStatus: "loading",
		keys:          defaultSummaryKeyMap(),
		palette:       p,
		width:         width,
		height:        height,
	}
	m.resize()
	return m
}

func (m *summaryPopupModel) Init() tea.Cmd {
	if m.summaryGetter == nil {
		m.summaryStatus = "unavailable"
		m.refresh()
		return nil
	}
	m.refresh()
	return loadSummaryCmd(m.ctx, m.summaryGetter, m.session.GetSessionId())
}

func (m *summaryPopupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		m.refresh()
	case summaryLoadedMsg:
		if msg.err != nil {
			m.summaryStatus = "error"
		} else {
			m.summary = msg.summary
			m.summaryStatus = "loaded"
		}
		m.refresh()
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Close):
			m.close()
			return m, func() tea.Msg { return closeSummaryMsg{} }
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *summaryPopupModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(m.palette.section)
	frameStyle := lipgloss.NewStyle().
		Width(m.popupWidth()).
		Height(m.popupHeight()).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.palette.accent).
		Padding(0, 1)

	header := titleStyle.Render(fmt.Sprintf("Summary  %s", sanitize(m.session.GetSessionId())))
	footer := lipgloss.NewStyle().
		Faint(true).
		Render("j/k or arrows: scroll  q/esc: close")

	bodyHeight := m.popupHeight() - lipgloss.Height(header) - lipgloss.Height(footer) - 2
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	m.viewport.Height = bodyHeight

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		m.viewport.View(),
		"",
		footer,
	)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		frameStyle.Render(content),
	)
}

func (m *summaryPopupModel) popupWidth() int {
	width := m.width * 70 / 100
	if width > 100 {
		width = 100
	}
	if width < 48 {
		width = min(m.width-2, 48)
	}
	if width < 24 {
		width = 24
	}
	return width
}

func (m *summaryPopupModel) popupHeight() int {
	height := m.height * 70 / 100
	if height > 32 {
		height = 32
	}
	if height < 12 {
		height = min(m.height-2, 12)
	}
	if height < 8 {
		height = 8
	}
	return height
}

func (m *summaryPopupModel) resize() {
	bodyWidth := m.popupWidth() - 4
	if bodyWidth < 20 {
		bodyWidth = 20
	}
	bodyHeight := m.popupHeight() - 6
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	m.viewport.Width = bodyWidth
	m.viewport.Height = bodyHeight
}

func (m *summaryPopupModel) refresh() {
	faintSt := lipgloss.NewStyle().Faint(true)

	switch m.summaryStatus {
	case "loading":
		m.viewport.SetContent(faintSt.Render("Loading summary..."))
	case "error":
		m.viewport.SetContent(faintSt.Render("Summary unavailable."))
	case "unavailable":
		m.viewport.SetContent(faintSt.Render("Summary service is not configured."))
	case "loaded":
		content := ""
		if m.summary != nil {
			content = m.summary.GetContent()
			if content == "" {
				content = m.summary.GetEnhancedSummary().GetDetailedDescription()
			}
		}

		var parts []string
		if strings.TrimSpace(content) != "" {
			parts = append(parts, renderMarkdownForTerminal(content, m.viewport.Width, m.palette))
		}
		if enh := m.summary.GetEnhancedSummary(); enh != nil && len(enh.GetCommands()) > 0 {
			parts = append(parts, renderTimeline(enh, m.viewport.Width, m.palette))
		}
		if len(parts) == 0 {
			m.viewport.SetContent(faintSt.Render("No summary available."))
			return
		}
		m.viewport.SetContent(strings.Join(parts, "\n\n"))
	default:
		m.viewport.SetContent("")
	}
}

func (m *summaryPopupModel) close() {
	m.cancel()
}

func loadSummaryCmd(ctx context.Context, getter SummaryGetter, sessionID string) tea.Cmd {
	return func() tea.Msg {
		resp, err := getter.GetSummary(ctx, &summarizerv1pb.GetSummaryRequest{SessionId: sessionID})
		if err != nil {
			return summaryLoadedMsg{err: err}
		}
		return summaryLoadedMsg{summary: resp.GetSummary()}
	}
}

type summaryKeyMap struct {
	Close key.Binding
}

func defaultSummaryKeyMap() summaryKeyMap {
	return summaryKeyMap{
		Close: key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q", "close")),
	}
}

// renderTimeline builds a timeline section from EnhancedSummary commands.
// Each entry shows the start offset, title, and optional subtitle. Commands
// listed in NotableCommandIndexes are prefixed with "*" and their risk level
// is shown in color.
func renderTimeline(enh *summarizerv1pb.EnhancedSummary, width int, p palette) string {
	commands := enh.GetCommands()
	if len(commands) == 0 {
		return ""
	}

	notableIdxs := enh.GetNotableCommandIndexes()
	notableSet := make(map[int]bool, len(notableIdxs))
	for _, idx := range notableIdxs {
		notableSet[int(idx)] = true
	}

	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(p.section)
	faintStyle := lipgloss.NewStyle().Faint(true)
	boldStyle := lipgloss.NewStyle().Bold(true)

	const offsetWidth = 7 // "mm:ss  "
	const markerWidth = 2 // "* " or "  "
	const riskWidth = 10  // "  CRITICAL" + padding
	subtitleIndent := strings.Repeat(" ", markerWidth+offsetWidth)
	titleMaxWidth := width - markerWidth - offsetWidth
	titleMaxWidthNotable := titleMaxWidth - riskWidth

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", sectionStyle.Render("Timeline"))

	for i, cmd := range commands {
		title := sanitize(cmd.GetTimelineTitle())
		if title == "" {
			title = sanitize(cmd.GetCommand())
		}
		if title == "" {
			continue
		}

		offset := "     "
		if d := cmd.GetStartOffset(); d != nil {
			total := d.AsDuration().Round(time.Second)
			m := int(total.Minutes())
			s := int(total.Seconds()) % 60
			offset = fmt.Sprintf("%02d:%02d", m, s)
		}

		isNotable := notableSet[i]
		marker := "  "
		titleStyle := faintStyle
		maxW := titleMaxWidth
		if isNotable {
			marker = "* "
			titleStyle = boldStyle
			maxW = titleMaxWidthNotable
		}
		if maxW > 0 && len(title) > maxW {
			title = title[:maxW-1] + "…"
		}

		line := fmt.Sprintf("%s%s  %s", marker, offset, titleStyle.Render(title))
		if isNotable && cmd.GetRiskLevel() != summarizerv1pb.RiskLevel_RISK_LEVEL_UNSPECIFIED {
			riskLabel := formatSeverityColored(cmd.GetRiskLevel())
			line = fmt.Sprintf("%-*s  %s", width-riskWidth, line, riskLabel)
		}
		fmt.Fprintf(&b, "%s\n", line)

		if sub := cmd.GetTimelineSubtitle(); sub != "" {
			fmt.Fprintf(&b, "%s%s\n", subtitleIndent, faintStyle.Render(sanitize(sub)))
		}
	}

	return strings.TrimRight(b.String(), "\n")
}
