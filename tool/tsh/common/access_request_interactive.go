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

package common

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ErrCanceled is returned when the user aborts via Esc or Ctrl+C.
var ErrCanceled = errors.New("prompt canceled by user")

type field int

const (
	fieldStart field = iota
	fieldDuration

	maxDays     = uint64((1<<63 - 1) / int64(24*time.Hour))
	minDuration = 1 * time.Second
	colorRed    = lipgloss.Color("9")
)

// AccessWindowForm manages the interactive Bubbletea form state.
type AccessWindowForm struct {
	// State internal to Bubbletea
	focus       field
	inputs      []textinput.Model
	parsedStart time.Time
	startValid  bool
	parsedDur   time.Duration
	durValid    bool
	canceled    bool
	submitted   bool
}

// NewAccessWindowForm initializes the form.
func NewAccessWindowForm() *AccessWindowForm {
	return &AccessWindowForm{}
}

// Run starts the Bubbletea program and blocks until the user confirms or quits.
// Returns (startTime, duration, error).
func (f *AccessWindowForm) Run(input io.Reader, output io.Writer) (time.Time, time.Duration, error) {
	f.initInputs()

	p := tea.NewProgram(f, tea.WithInput(input), tea.WithOutput(output))
	finalModel, err := p.Run()
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("bubbletea program error: %w", err)
	}

	m, ok := finalModel.(*AccessWindowForm)
	if !ok {
		return time.Time{}, 0, errors.New("invalid model returned")
	}

	if m.canceled {
		return time.Time{}, 0, ErrCanceled
	}

	if !m.submitted || !m.startValid || !m.durValid {
		return time.Time{}, 0, errors.New("form exited without valid input")
	}

	return m.parsedStart, m.parsedDur, nil
}

func (f *AccessWindowForm) initInputs() {
	f.inputs = make([]textinput.Model, 2)

	// Start Input
	start := textinput.New()
	start.Placeholder = "now, +30m, 17:00, 05:00PM or YYYY-MM-DD HH:MM[:SS+-07:00]"
	start.Prompt = " Start Time: "
	start.CharLimit = 70
	start.Width = 70
	start.Focus()
	f.inputs[fieldStart] = start

	// Duration Input
	dur := textinput.New()
	dur.Placeholder = "e.g. 15m, 1h, 8h"
	dur.Prompt = " Duration:   "
	dur.CharLimit = 40
	dur.Width = 40
	f.inputs[fieldDuration] = dur

	f.validate()
}

func (f *AccessWindowForm) validate() {
	now := time.Now()

	// Validate Start Time
	startStr := strings.TrimSpace(f.inputs[fieldStart].Value())
	if t, err := parseStartExpression(startStr, now); err == nil {
		f.parsedStart = t
		f.startValid = true
	} else {
		f.startValid = false
	}

	// Validate Duration
	durStr := strings.TrimSpace(f.inputs[fieldDuration].Value())
	if d, err := parseExtendedDuration(durStr); err == nil && d >= minDuration {
		f.parsedDur = d
		f.durValid = true
	} else {
		f.durValid = false
	}
}

func (f *AccessWindowForm) Init() tea.Cmd {
	return textinput.Blink
}

func (f *AccessWindowForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			f.canceled = true
			return f, tea.Quit

		case "tab", "shift+tab":
			if f.focus == fieldStart {
				f.focus = fieldDuration
				f.inputs[fieldStart].Blur()
				f.inputs[fieldDuration].Focus()
			} else {
				f.focus = fieldStart
				f.inputs[fieldDuration].Blur()
				f.inputs[fieldStart].Focus()
			}

		case "up":
			if f.focus == fieldStart {
				f.inputs[fieldStart].CursorStart()
			}
			if f.focus == fieldDuration {
				f.inputs[fieldDuration].CursorStart()
			}

		case "down":
			if f.focus == fieldStart {
				f.inputs[fieldStart].CursorEnd()
			}
			if f.focus == fieldDuration {
				f.inputs[fieldDuration].CursorEnd()
			}

		case "enter":
			if f.startValid && f.durValid {
				f.submitted = true
				return f, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	focusedInput := &f.inputs[f.focus]
	*focusedInput, cmd = focusedInput.Update(msg)

	f.validate()
	return f, cmd
}

func (f *AccessWindowForm) View() string {
	if f.submitted || f.canceled {
		return "" // Clear interactive UI on finish
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", lipgloss.NewStyle().
		Bold(true).
		Render("Request Access Window"))

	for i := range f.inputs {
		fmt.Fprintln(&b, f.inputs[i].View())
	}

	fmt.Fprintf(&b, "\n%s\n", lipgloss.
		NewStyle().
		Faint(true).
		Render("───────────── PREVIEW ─────────────"))

	if f.startValid {
		fmt.Fprintf(&b, "  Start:  %s\n", f.parsedStart.Format("Mon, Jan 2, 2006 at 15:04 MST"))
	} else {
		fmt.Fprintf(&b, "  Start:  %s\n", lipgloss.NewStyle().
			Foreground(colorRed).
			Render("Invalid start time"))
	}

	if f.startValid && f.durValid {
		fmt.Fprintf(&b, "  Ends:   %s\n", f.parsedStart.Add(f.parsedDur).Format("Mon, Jan 2, 2006 at 15:04 MST"))
		fmt.Fprintf(&b, "  Window: %s\n", f.parsedDur)
	} else if !f.durValid {
		fmt.Fprintf(&b, "  Ends:   %s\n", lipgloss.NewStyle().
			Foreground(colorRed).
			Render("Invalid duration"))
	} else {
		fmt.Fprintf(&b, "  Ends:   %s\n", lipgloss.NewStyle().
			Foreground(colorRed).
			Render("Invalid start time"))
	}

	fmt.Fprintf(&b, "\n%s\n", lipgloss.NewStyle().Faint(true).
		Render("[Tab] Switch Field  •  [Up/Down] Start/End of line  •  [Enter] Confirm  •  [Esc] Cancel"))

	return b.String()
}

func parseExtendedDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		daysStr := strings.TrimSuffix(s, "d")
		days, err := strconv.ParseUint(daysStr, 10, 64)
		if err != nil || days == 0 || days > maxDays {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// parseStartExpression parses a user string into a time.Time using ONLY stdlib.
func parseStartExpression(input string, now time.Time) (time.Time, error) {
	s := strings.TrimSpace(input)
	if s == "" || strings.EqualFold(s, "now") {
		return now, nil
	}

	// Check relative duration offset (e.g., "+30m", "+2h", "+1d" or "30m", "2h")
	offsetStr := strings.TrimPrefix(s, "+")
	if d, err := parseExtendedDuration(offsetStr); err == nil && d >= 0 {
		return now.Add(d), nil
	}

	// Check time-of-day format (e.g., "15:00", "09:30:00", "3:45PM", "3:45:30PM")
	for _, layout := range []string{"15:04", "15:04:05", "3:04PM", "3:04:05PM"} {
		if t, err := time.ParseInLocation(layout, s, now.Location()); err == nil {
			// Construct full time using today's YYYY-MM-DD
			target := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), 0, now.Location())

			// If target time is earlier today, rollover to tomorrow
			if target.Before(now) {
				target = target.AddDate(0, 0, 1)
			}
			return target, nil
		}
	}

	// Check full datetime standard formats
	formats := []string{
		"2006",
		"2006-01",
		"2006-01-02 15",
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05Z07",
		"2006-01-02 15:04:05Z0700",
		"2006-01-02 15:04:05Z070000",
		"2006-01-02 15:04:05Z07:00:00",
		"2006-01-02T15",
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z07",
		"2006-01-02T15:04:05Z0700",
		"2006-01-02T15:04:05Z070000",
		"2006-01-02T15:04:05Z07:00:00",
		time.RFC822,
		time.RFC1123,
		time.RFC3339,
		time.DateOnly, // "2006-01-02" -> starts at midnight
	}

	for _, format := range formats {
		if t, err := time.ParseInLocation(format, s, now.Location()); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized format: %q", s)
}
