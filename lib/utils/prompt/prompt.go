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

package prompt

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitational/trace"
)

// ErrInterrupted is an error when prompt is interrupted with ctrl-c or a signal.
var ErrInterrupted = errors.New("interrupted")

// ErrCanceled is an error when quit option was selected.
var ErrCanceled = errors.New("canceled")

// SelectModel is a generic struct that holds the options, context, and state.
type SelectModel[T any] struct {
	caption     string
	options     []T
	cursor      int
	selected    *T
	renderRow   func(T) string
	quitting    bool
	interrupted bool
}

// NewSelectPrompt initializes the generic model with options, a renderer, and a context.
func NewSelectPrompt[T any](caption string, options []T, renderRow func(T) string) SelectModel[T] {
	if renderRow == nil {
		renderRow = func(t T) string {
			return fmt.Sprintf("%v", t)
		}
	}

	return SelectModel[T]{
		caption:   caption,
		options:   options,
		renderRow: renderRow,
	}
}

func (m SelectModel[T]) Init() tea.Cmd {
	return nil
}

func (m SelectModel[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.interrupted = true
			return m, tea.Quit
		case "q":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.options) > 0 {
				m.selected = &m.options[m.cursor]
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m SelectModel[T]) View() string {
	if m.interrupted {
		return "Prompt interrupted.\n"
	}
	if m.quitting {
		return "Prompt canceled by user.\n"
	}
	if m.selected != nil {
		return fmt.Sprintf("Selected: %s\n", m.renderRow(*m.selected))
	}

	sb := &strings.Builder{}

	fmt.Fprintf(sb, "%s:\n\n", m.caption)

	for i, option := range m.options {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		fmt.Fprintf(sb, "%s %s\n", cursor, m.renderRow(option))
	}

	fmt.Fprintln(sb, "\n(Use arrows or hjkl to navigate, enter to select, q to quit)")
	return sb.String()
}

// Run helper handles running the program and extracting the generic result.
func (m SelectModel[T]) Run(ctx context.Context) (T, error) {
	if err := ctx.Err(); err != nil {
		return *new(T), trace.Wrap(err)
	}

	if len(m.options) == 0 {
		return *new(T), trace.BadParameter("no options provided")
	}

	p := tea.NewProgram(m, tea.WithContext(ctx))
	finalModel, err := p.Run()
	if err != nil {
		return *new(T), trace.Wrap(err)
	}

	typedModel := finalModel.(SelectModel[T])

	if typedModel.quitting {
		return *new(T), trace.Wrap(ErrCanceled)
	}

	if typedModel.interrupted || typedModel.selected == nil {
		return *new(T), trace.Wrap(ErrInterrupted)
	}

	return *typedModel.selected, nil
}
