/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package spinner

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// Spinner renders an inline animated spinner to a writer.
type Spinner struct {
	w     io.Writer
	model spinner.Spinner
	style lipgloss.Style

	finalMessage chan string
	done         chan struct{}
	stopOnce     sync.Once
}

// New creates and starts an inline spinner that writes to w.
// Call Stop to replace the spinner line with a final message.
func New(w io.Writer, msg string) *Spinner {
	s := &Spinner{
		w:            w,
		finalMessage: make(chan string, 1),
		done:         make(chan struct{}),
		// ANSI color "6" is cyan. style and model are hard-coded atm but can
		// potentially become options for spinners.
		style: lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true),
		model: spinner.MiniDot,
	}
	go s.run(msg)
	return s
}

func (s *Spinner) run(msg string) {
	defer close(s.done)
	ticker := time.NewTicker(s.model.FPS)
	defer ticker.Stop()
	i := 0
	for {
		select {
		case final := <-s.finalMessage:
			fmt.Fprintf(s.w, "\r%s\r", strings.Repeat(" ", len(msg)+2))
			if final != "" {
				fmt.Fprintln(s.w, final)
			}
			return
		case <-ticker.C:
			frame := s.model.Frames[i%len(s.model.Frames)]
			fmt.Fprintf(s.w, "\r%s %s", s.style.Render(frame), msg)
			i++
		}
	}
}

// Stop clears the spinner line.
func (s *Spinner) Stop() {
	s.StopWithMessage("")
}

// StopWithMessage clears the spinner line and prints msg.
func (s *Spinner) StopWithMessage(msg string) {
	s.stopOnce.Do(func() {
		s.finalMessage <- msg
		<-s.done
	})
}
