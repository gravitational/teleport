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

package oidc

import (
	"io"
	"net/http"

	"github.com/gravitational/trace"
)

// OIDCRoundTripper wrapps the [http.RoundTripper] provided to the
// [rp.RelyingParty] to prevent reading response bodies that exceed
// [maxDataSize].
//
// TODO(tross): remove this when https://github.com/zitadel/oidc/issues/738
// is resolved.
type OIDCRoundTripper struct {
	rt http.RoundTripper
}

func (l *OIDCRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := l.rt.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	return &http.Response{
		Status:           resp.Status,
		StatusCode:       resp.StatusCode,
		Proto:            resp.Proto,
		ProtoMajor:       resp.ProtoMajor,
		ProtoMinor:       resp.ProtoMinor,
		Header:           resp.Header,
		Body:             newLimitReadCloser(resp.Body, resp.Body),
		ContentLength:    resp.ContentLength,
		TransferEncoding: resp.TransferEncoding,
		Close:            resp.Close,
		Uncompressed:     resp.Uncompressed,
		Trailer:          resp.Trailer,
		Request:          resp.Request,
		TLS:              resp.TLS,
	}, nil
}

func (l *OIDCRoundTripper) CloseIdleConnections() {
	type closeIdler interface {
		CloseIdleConnections()
	}
	if tr, ok := l.rt.(closeIdler); ok {
		tr.CloseIdleConnections()
	}
}

const maxDataSize = 1024 * 1024

// limitReadCloser is an [io.ReadCloser] that limits reading
// less than [maxDataSize] from the reader.
type limitReadCloser struct {
	n      int64
	reader io.Reader
	closer io.Closer
}

func newLimitReadCloser(reader io.Reader, closer io.Closer) *limitReadCloser {
	return &limitReadCloser{
		n:      maxDataSize + 1,
		reader: reader,
		closer: closer,
	}
}

func (r *limitReadCloser) Read(p []byte) (int, error) {
	if r.n <= 0 {
		// Return an error immediately without attempting to drain the
		// connection: a malfunctioning or malicious remote server could slowly
		// keep writing bytes indefinitely and block our reader. If it exceeds
		// our max size, just kill the connection.
		return 0, trace.Errorf("response exceeds maximum size of %d bytes", maxDataSize)
	}

	if int64(len(p)) > r.n {
		p = p[0:r.n]
	}
	n, err := r.reader.Read(p)
	r.n -= int64(n)
	return n, err
}

func (r *limitReadCloser) Close() error {
	return r.closer.Close()
}

// NewOIDCRoundTripper returns a round tripper that enforces a maximum response
// size limit ([maxDataSize]).
func NewOIDCRoundTripper(rt http.RoundTripper) http.RoundTripper {
	return &OIDCRoundTripper{
		rt: rt,
	}
}
