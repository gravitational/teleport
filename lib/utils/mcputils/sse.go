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

package mcputils

import (
	"bufio"
	"context"
	"strings"

	"github.com/gravitational/trace"
)

type SSEMessage struct {
	Event string
	Data  string
}

func ReadMessageFromSSEBody(ctx context.Context, br *bufio.Reader) (*SSEMessage, error) {
	var event, data string
	for {
		if ctx.Err() != nil {
			return nil, trace.Wrap(ctx.Err())
		}
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Remove only newline markers
		line = strings.TrimRight(line, "\r\n")

		// Empty line means end of event
		if line == "" {
			if data != "" {
				if event == "" {
					event = "message"
				}
				return &SSEMessage{Event: event, Data: data}, nil
			}
			continue
		}

		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
}
