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

package pam

import (
	"errors"
	"io"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/session/terminal"
)

const challengeName = "PAM"

// limitedLineReader is a reader that reads lines. Lines longer than the max
// line size are truncated. limitedLineReader does not buffer.
type limitedLineReader struct {
	io.Reader
	maxLineSize int
}

func (r *limitedLineReader) readLine() (string, error) {
	b := make([]byte, r.maxLineSize)
	const delim = '\n'
	// Read one byte at a time to avoid consuming too much from the reader.
	for i := range r.maxLineSize {
		_, err := r.Read(b[i : i+1])
		if errors.Is(err, io.EOF) {
			return string(b[:i]), nil
		}
		if err != nil {
			return "", trace.Wrap(err)
		}
		if b[i] == delim {
			return string(b[:i+1]), nil
		}
	}
	return string(b), nil
}

// pamMessage is a Go equivalent of struct pam_message.
type pamMessage struct {
	style int
	msg   string
}

func formatOutput(s string) string {
	// Replace \n with \r\n so the message correctly aligned.
	return strings.NewReplacer("\r\n", "\r\n", "\n", "\r\n").Replace(
		terminal.StripTerminalControlSequences(
			strings.TrimSpace(s),
		),
	)
}

// printHandler is a [handler] that copies PAM message directly to/from the
// standard files.
type printHandler struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func (h *printHandler) converse(messages []pamMessage) ([]*string, error) {
	responses := make([]*string, 0, len(messages))
	// Limit the reader in case stdin is from /dev/zero or other infinite
	// source.
	reader := &limitedLineReader{Reader: h.stdin, maxLineSize: maxResponseSize}
	for _, message := range messages {
		formattedMsg := []byte(formatOutput(message.msg) + "\r\n")
		switch message.style {
		case styleTextInfo, styleErrorMessage:
			f := h.stdout
			if message.style == styleErrorMessage {
				f = h.stderr
			}
			_, err := f.Write(formattedMsg)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			responses = append(responses, nil)
		case stylePromptEchoOff, stylePromptEchoOn:
			_, err := h.stdout.Write(formattedMsg)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			answer, err := reader.readLine()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			responses = append(responses, new(strings.TrimRight(answer, "\r\n")))
		default:
			return nil, trace.BadParameter("unknown message style: %v", message.style)
		}
	}
	return responses, nil
}

// sshAuthHandler is a [handler] that formats the PAM conversation for use
// with SSH as a keyboard-interactive auth method.
type sshAuthHandler struct {
	challenge ssh.KeyboardInteractiveChallenge
}

func (h *sshAuthHandler) sendChallenge(instructions, questions []string, echos []bool) ([]*string, error) {
	if len(instructions) == 0 && len(questions) == 0 {
		return nil, nil
	}

	responses := make([]*string, 0, len(instructions)+len(questions))
	answers, err := h.challenge(challengeName, strings.Join(instructions, "\r\n"), questions, echos)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(answers) != len(questions) {
		return nil, trace.Errorf("challenge returned wrong number of answers")
	}
	for range instructions {
		responses = append(responses, nil)
	}

	for _, answer := range answers {
		answer = strings.TrimRight(answer, "\r\n")
		if len(answer) > maxResponseSize {
			answer = answer[:maxResponseSize]
		}
		responses = append(responses, new(answer))
	}

	return responses, nil
}

func (h *sshAuthHandler) converse(messages []pamMessage) ([]*string, error) {
	responses := make([]*string, 0, len(messages))
	instructions := make([]string, 0, len(messages))
	questions := make([]string, 0, len(messages))
	echos := make([]bool, 0, len(messages))
	gotPrompt := false

	for _, msg := range messages {
		switch msg.style {
		case stylePromptEchoOff, stylePromptEchoOn:
			gotPrompt = true
			questions = append(questions, formatOutput(msg.msg))
			echos = append(echos, msg.style == stylePromptEchoOn)
		case styleTextInfo, styleErrorMessage:
			if gotPrompt {
				// This print should be the start of a new challenge. Send current questions.
				resp, err := h.sendChallenge(instructions, questions, echos)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				gotPrompt = false
				responses = append(responses, resp...)

				// Reset instructions and questions.
				instructions = make([]string, 0, len(messages))
				questions = make([]string, 0, len(messages))
				echos = make([]bool, 0, len(messages))
			}
			instructions = append(instructions, formatOutput(msg.msg))
		default:
			return nil, trace.BadParameter("unknown message style: %v", msg.style)
		}
	}

	resp, err := h.sendChallenge(instructions, questions, echos)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return append(responses, resp...), nil
}
