/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package snowflake

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

func writeResponse(resp *http.Response, newResp []byte) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	if isGzipEncoded(resp) {
		newGzBody := gzip.NewWriter(buf)
		defer newGzBody.Close()

		if _, err := newGzBody.Write(newResp); err != nil {
			return nil, trace.Wrap(err)
		}

		if err := newGzBody.Close(); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		buf.Write(newResp)
	}
	return buf, nil
}

// isGzipEncoded returns true if the body should be gzip compressed.
func isGzipEncoded(resp *http.Response) bool {
	return strings.Contains(resp.Header.Get("Content-Encoding"), "gzip")
}

func copyRequest(ctx context.Context, req *http.Request, body io.Reader) (*http.Request, error) {
	reqCopy, err := http.NewRequestWithContext(ctx, req.Method, req.URL.String(), body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reqCopy.Header = req.Header.Clone()

	return reqCopy, nil
}

func readRequestBody(req *http.Request) ([]byte, error) {
	defer req.Body.Close()

	body, err := utils.ReadAtMost(req.Body, teleport.MaxHTTPRequestSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return maybeReadGzip(&req.Header, body, teleport.MaxHTTPRequestSize)
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()

	body, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return maybeReadGzip(&resp.Header, body, teleport.MaxHTTPResponseSize)
}

// maybeReadGzip checks if the body is gzip encoded and returns decoded version.
// To determine gzip encoding the beginning of body message is being checked
// instead of HTTP header and the second one was less reliable during testing.
func maybeReadGzip(headers *http.Header, body []byte, limit int64) ([]byte, error) {
	gzipMagic := []byte{0x1f, 0x8b, 0x08}

	// Check if the body is gzip encoded. Alternative here could check
	// Content-Encoding header, but for some reason during testing header check
	// didn't always work.
	if !bytes.HasPrefix(body, gzipMagic) {
		return body, nil
	}

	bodyGZ, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer bodyGZ.Close()

	body, err = utils.ReadAtMost(bodyGZ, limit)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Make sure that the content-encoding is correct.
	headers.Set("Content-Encoding", "gzip")

	return body, nil
}
