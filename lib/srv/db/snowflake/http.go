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

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
)

func writeResponse(resp *http.Response, newResp []byte) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	if resp.Header.Get("Content-Encoding") == "gzip" {
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

func copyRequest(ctx context.Context, req *http.Request, body io.Reader) (*http.Request, error) {
	reqCopy, err := http.NewRequestWithContext(ctx, req.Method, req.URL.String(), body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reqCopy.Header = req.Header.Clone()

	return reqCopy, nil
}

func readRequestBody(req *http.Request) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(req.Body, teleport.MaxHTTPRequestSize))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return readBody(req.Header, body)
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, teleport.MaxHTTPRequestSize))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return readBody(resp.Header, body)
}

func readBody(headers http.Header, body []byte) ([]byte, error) {
	if headers.Get("Content-Encoding") == "gzip" {
		bodyGZ, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer bodyGZ.Close()

		body, err = io.ReadAll(bodyGZ)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return body, nil
}
