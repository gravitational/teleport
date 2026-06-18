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

package srv

// Control bytes recognized by lineBuffer while reconstructing the command line
// from raw PTY input.
const (
	byteCR         = '\r' // carriage return, line terminator
	byteLF         = '\n' // line feed, line terminator
	byteBackspace  = 0x7f // DEL, sent by most terminals for Backspace
	byteBackspace2 = 0x08 // BS, alternate Backspace
	byteCtrlU      = 0x15 // clear the current line
	byteCtrlC      = 0x03 // abort the current line
)

// lineBuffer reconstructs a best-effort command line from the raw bytes a user
// types into an interactive PTY. It interprets the common terminal editing
// controls (Backspace, Ctrl-U, Ctrl-C) and line terminators so a moderator or
// AI reviewer can inspect the prospective command before Enter reaches the
// shell.
//
// Reconstruction is best-effort by design: shell-side line editing such as
// tab-completion, history recall (up-arrow), and reverse-i-search is NOT
// mirrored here, because those transformations happen inside the remote shell
// and are not visible in the input byte stream. The reconstructed command is
// therefore an approximation used only for the approval gate; the authoritative
// record of what actually executed is reconciled against the BPF
// session.command audit events.
type lineBuffer struct {
	buf []byte
}

// feedLines consumes input bytes and returns every command line completed
// within them, in order. A trailing partial line (no terminator yet) remains
// buffered for the next call. Blank lines (a bare terminator) are skipped and
// not returned. This is the multi-line-safe entry point used by the approval
// gate so that a single write containing several newlines (e.g. a pasted block)
// gates each command rather than silently dropping all but the last.
func (lb *lineBuffer) feedLines(p []byte) []string {
	var lines []string
	for _, b := range p {
		if line, terminated := lb.feedByte(b); terminated && line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// feedByte applies a single input byte to the buffer following the terminal
// editing rules (Backspace, Ctrl-U, Ctrl-C, line terminators). It returns
// (line, true) when the byte is a line terminator, where line is the contents
// of the buffer just before it was reset (the empty string for a blank line).
// For all non-terminating bytes it returns ("", false). It is the shared
// per-byte primitive used by feedLines.
func (lb *lineBuffer) feedByte(b byte) (string, bool) {
	switch b {
	case byteCR, byteLF:
		line := string(lb.buf)
		lb.buf = lb.buf[:0]
		return line, true
	case byteBackspace, byteBackspace2:
		if len(lb.buf) > 0 {
			lb.buf = lb.buf[:len(lb.buf)-1]
		}
	case byteCtrlU:
		lb.buf = lb.buf[:0]
	case byteCtrlC:
		lb.buf = lb.buf[:0]
	default:
		lb.buf = append(lb.buf, b)
	}
	return "", false
}
