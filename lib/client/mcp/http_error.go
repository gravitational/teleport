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

package mcp

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gravitational/teleport/lib/utils/mcputils"
)

const httpServerErrorBodyLimit = 256

// HTTPServerError describes an MCP server HTTP response error.
type HTTPServerError struct {
	StatusCode int
	Origin     string
	Body       string
}

func (e *HTTPServerError) Error() string {
	var sb strings.Builder
	sb.WriteString("request failed with status ")
	sb.WriteString(strconv.Itoa(e.StatusCode))
	if e.Origin != "" {
		sb.WriteString(" (reported by ")
		sb.WriteString(e.Origin)
		sb.WriteByte(')')
	}
	if e.Body != "" {
		sb.WriteString(": ")
		sb.WriteString(e.Body)
	}
	return sb.String()
}

type httpServerErrorRoundTripper struct {
	base http.RoundTripper
}

func (t *httpServerErrorRoundTripper) CloseIdleConnections() {
	type closeIdler interface {
		CloseIdleConnections()
	}
	if base, ok := t.base.(closeIdler); ok {
		base.CloseIdleConnections()
	}
}

func (t *httpServerErrorRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || resp.StatusCode < http.StatusInternalServerError {
		return resp, err
	}
	// Only turn responses annotated by Teleport into transport errors. Leave
	// unmarked response bodies intact for the MCP client to handle.
	origin := resp.Header.Get(mcputils.TeleportErrorOriginHeader)
	if origin == "" {
		return resp, nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, httpServerErrorBodyLimit))
	resp.Body.Close()
	return nil, &HTTPServerError{
		StatusCode: resp.StatusCode,
		Origin:     origin,
		Body:       strings.Join(strings.Fields(string(body)), " "),
	}
}
