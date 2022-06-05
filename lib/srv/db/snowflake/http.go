/*

 Copyright 2022 Gravitational, Inc.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.

*/

package snowflake

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
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

	body, err := io.ReadAll(io.LimitReader(req.Body, teleport.MaxHTTPRequestSize))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return maybeReadGzip(&req.Header, body)
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, teleport.MaxHTTPRequestSize))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return maybeReadGzip(&resp.Header, body)
}

// maybeReadGzip checks if the body is gzip encoded and returns decoded version.
// To determine gzip encoding the beginning of body message is being checked
// instead of HTTP header and the second one was less reliable during testing.
func maybeReadGzip(headers *http.Header, body []byte) ([]byte, error) {
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

	body, err = io.ReadAll(bodyGZ)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Make sure that the content-encoding is correct.
	headers.Set("Content-Encoding", "gzip")

	return body, nil
}
