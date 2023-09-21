/*
Copyright 2023 Gravitational, Inc.

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

package clickhouse

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/andybalholm/brotli"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/utils"
)

const (

	// ClubHouse HTTP headers that allows HTTP for x509 Auth.
	// Reference: https://clickhouse.com/docs/en/guides/sre/ssl-user-auth#4-testing-http
	headerClickHouseUser    = "X-ClickHouse-User"
	headerClickHouseSSLAuth = "X-ClickHouse-SSL-Certificate-Auth"
	enableVal               = "on"
)

func (e *Engine) handleHTTPConnection(ctx context.Context, sessionCtx *common.Session) error {
	tr, err := e.getTransport(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	clientConnReader := bufio.NewReader(e.clientConn)
	for {
		req, err := http.ReadRequest(clientConnReader)
		if err != nil {
			return trace.Wrap(err)
		}
		query, err := getQuery(req)
		if err != nil {
			return trace.Wrap(err)
		}

		queryEvent := common.Query{
			Query:      query,
			Parameters: []string{fmt.Sprintf("url=%s", req.URL.String())},
		}

		e.Audit.OnQuery(e.Context, sessionCtx, queryEvent)

		if err := e.handleRequest(req, sessionCtx); err != nil {
			return trace.Wrap(err)
		}

		resp, err := tr.RoundTrip(req)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := e.writeResp(resp); err != nil {
			return trace.Wrap(err)
		}
	}
}

func handleCompression(body []byte, compression string) ([]byte, error) {
	var (
		r   io.ReadCloser
		err error
	)

	switch compression {
	case "", clickhouse.CompressionNone.String():
		return body, nil
	case clickhouse.CompressionGZIP.String():
		r, err = gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer r.Close()
	case clickhouse.CompressionDeflate.String():
		r = flate.NewReader(bytes.NewReader(body))
		defer r.Close()
	case clickhouse.CompressionBrotli.String():
		r = io.NopCloser(brotli.NewReader(bytes.NewReader(body)))
		defer r.Close()
	default:
		return nil, trace.BadParameter("clickHouse engine unsupported compression method %s", compression)
	}

	body, err = io.ReadAll(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return body, nil
}

func getQuery(req *http.Request) (string, error) {
	body, err := utils.GetAndReplaceRequestBody(req)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if enc := req.Header.Get("Content-Encoding"); enc != "" {
		body, err = handleCompression(body, enc)
		if err != nil {
			return "", trace.Wrap(err)
		}
	}

	var bodyQuery string
	if req.Header.Get("Content-Type") == "application/octet-stream" {
		bodyQuery = hex.EncodeToString(body)
	} else {
		bodyQuery = string(body)
	}

	if urlQuery := req.URL.Query().Get("query"); urlQuery != "" {
		if bodyQuery == "" {
			return urlQuery, nil
		}
		return fmt.Sprintf("%s %s", urlQuery, bodyQuery), nil
	}
	return bodyQuery, nil
}

func (e *Engine) writeResp(resp *http.Response) error {
	defer resp.Body.Close()
	if err := resp.Write(e.clientConn); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (e *Engine) handleRequest(req *http.Request, sessionCtx *common.Session) error {
	uri, err := url.Parse(sessionCtx.Database.GetURI())
	if err != nil {
		return trace.Wrap(err)
	}

	req.URL.Scheme = "https"
	req.URL.Host = uri.Host

	// Delete Headers set by a ClickHouse client.
	req.Header.Del("Authorization")

	// Set ClickHouse Headers to enforce x509 auth for HTTP protocol.
	req.Header.Set(headerClickHouseSSLAuth, enableVal)
	req.Header.Set(headerClickHouseUser, sessionCtx.DatabaseUser)
	return nil

}

func (e *Engine) sendErrorHTTP(err error) {
	statusCode := http.StatusInternalServerError
	if trace.IsAccessDenied(err) {
		statusCode = http.StatusUnauthorized
	}

	response := &http.Response{
		ProtoMajor: 1,
		ProtoMinor: 1,
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(err.Error())),
	}
	if err := response.Write(e.clientConn); err != nil {
		return
	}
}

func (e *Engine) getTransport(ctx context.Context) (*http.Transport, error) {
	transport, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig, err := e.Auth.GetTLSConfig(ctx, e.sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	transport.TLSClientConfig = tlsConfig
	return transport, nil
}
