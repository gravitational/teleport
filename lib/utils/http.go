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

package utils

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"strings"

	"github.com/gravitational/trace"
)

// GetAndReplaceRequestBody returns the request body and replaces the drained
// body reader with an [io.NopCloser] allowing for further body processing by
// http transport.
// If memory exhaustion is a concern, it is the caller's responsibility to wrap
// the request body in an [io.LimitReader] prior to calling this function.
func GetAndReplaceRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil || req.Body == http.NoBody {
		return []byte{}, nil
	}
	defer req.Body.Close()

	payload, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Replace the drained body with io.NopCloser reader allowing for further request processing by HTTP transport.
	req.Body = io.NopCloser(bytes.NewReader(payload))
	return payload, nil
}

// GetAndReplaceResponseBody returns the response body and replaces the drained
// body reader with [io.NopCloser] allowing for further body processing.
// If memory exhaustion is a concern, it is the caller's responsibility to wrap
// the response body in an [io.LimitReader] prior to calling this function.
func GetAndReplaceResponseBody(response *http.Response) ([]byte, error) {
	if response.Body == nil {
		return []byte{}, nil
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response.Body = io.NopCloser(bytes.NewReader(payload))
	return payload, nil
}

// ReplaceRequestBody drains the old request body and replaces it with a new one.
func ReplaceRequestBody(req *http.Request, newBody io.ReadCloser) error {
	if req.Body != nil {
		defer req.Body.Close()
		// drain and discard the request body to allow connection reuse.
		// No need to enforce a max request size, nor rely on callers to do so,
		// since we do not buffer the entire request body.
		_, err := io.Copy(io.Discard, req.Body)
		if err != nil && !errors.Is(err, io.EOF) {
			return trace.Wrap(err)
		}
	}
	req.Body = newBody
	return nil
}

// RenameHeader moves all values from the old header key to the new header key.
func RenameHeader(header http.Header, oldKey, newKey string) {
	if oldKey == newKey {
		return
	}
	for _, value := range header.Values(oldKey) {
		header.Add(newKey, value)
	}
	header.Del(oldKey)
}

// IsRedirect returns true if the status code is a 3xx code.
func IsRedirect(code int) bool {
	if code >= http.StatusMultipleChoices && code <= http.StatusPermanentRedirect {
		return true
	}
	return false
}

// GetAnyHeader returns the first non-empty value by the provided keys.
func GetAnyHeader(header http.Header, keys ...string) string {
	for _, key := range keys {
		if value := header.Get(key); value != "" {
			return value
		}
	}
	return ""
}

// GetSingleHeader will return the header value for the key if there is exactly one value present.  If the header is
// missing or specified multiple times, an error will be returned.
func GetSingleHeader(headers http.Header, key string) (string, error) {
	values := headers.Values(key)
	if len(values) > 1 {
		return "", trace.BadParameter("multiple %q headers", key)
	} else if len(values) == 0 {
		return "", trace.NotFound("missing %q headers", key)
	} else {
		return values[0], nil
	}
}

// HTTPDoClient is an interface that defines the Do function of http.Client.
type HTTPDoClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// HTTPMiddleware defines a HTTP middleware.
type HTTPMiddleware func(next http.Handler) http.Handler

// ChainHTTPMiddlewares wraps an http.Handler with a list of middlewares. Inner
// middlewares should be provided before outer middlewares.
func ChainHTTPMiddlewares(handler http.Handler, middlewares ...HTTPMiddleware) http.Handler {
	if len(middlewares) == 0 {
		return handler
	}
	apply := middlewares[0]
	middlewares = middlewares[1:]
	if apply != nil {
		handler = apply(handler)
	}
	return ChainHTTPMiddlewares(handler, middlewares...)
}

// NoopHTTPMiddleware is a no-operation HTTPMiddleware that returns the
// original handler.
func NoopHTTPMiddleware(next http.Handler) http.Handler {
	return next
}

// MaxBytesReader returns an [io.ReadCloser] that wraps an [http.MaxBytesReader]
// to act as a shim for converting from [http.MaxBytesError] to
// [ErrLimitReached].
func MaxBytesReader(w http.ResponseWriter, r io.ReadCloser, n int64) io.ReadCloser {
	return &maxBytesReader{ReadCloser: http.MaxBytesReader(w, r, n)}
}

// maxBytesReader wraps an [http.MaxBytesReader] and converts any
// [http.MaxBytesError] to [ErrLimitReached].
type maxBytesReader struct {
	io.ReadCloser
}

func (m *maxBytesReader) Read(p []byte) (int, error) {
	n, err := m.ReadCloser.Read(p)

	// convert [http.MaxBytesError] to our limit error.
	var mbErr *http.MaxBytesError
	if errors.As(err, &mbErr) {
		return n, ErrLimitReached
	}
	return n, err
}

// Sensitive headers list based on team discussion and Akami's list of
// not-to-be-logged headers here:
//    https://techdocs.akamai.com/edge-diagnostics/reference/sensitive-headers
//
//  - Authorization
//  - Proxy-Authorization
//  - Set-Cookie
//  - Anything containing "API-Key"
//  - X-Amz-Security-Token

// sensitiveHeaderKeys is the list HTTP headers deemed to be too sensitive
// to be written to a log.
var sensitiveHeaderKeys = []string{
	"Authorization",
	"Cookie",
	"Proxy-Authorization",
	"Set-Cookie",
	"X-Amz-Security-Token",
	"X-Csrf-Token",
}

// sensitiveHeaderFragments is a list of suspect header fragments. If a header
// key contains any of these fragments it will be filtered out by
// SanitizeHeaders()
var sensitiveHeaderFragments = []string{
	"api-key",
}

// SanitizedHeaderValuer is a slog.LogValuer for http.Headers that will lazily
// filter out sensitive headers when logged
type SanitizedHeaderValuer http.Header

// Static assertion that SanitizedHeaderValuer implements slog.LogValuer
var _ slog.LogValuer = SanitizedHeaderValuer(nil)

// LogValue implements slog.LogValuer for SanitizedHeaderValuer. Headers will be
// formatted into a slog.Value as a string, omitting any "sensitive" headers.
func (h SanitizedHeaderValuer) LogValue() slog.Value {
	return slog.AnyValue(SanitizeHeaders(http.Header(h)))
}

// SanitizeHeaders formats the supplied HTTP headers as a string, omitting any
// "sensitive" headers that should not appear in a log.
func SanitizeHeaders(src http.Header) http.Header {
	// Preserve nil in case its important
	if src == nil {
		return nil
	}

	dst := maps.Clone(src)
	for _, k := range sensitiveHeaderKeys {
		dst.Del(k)
	}

nextkey:
	for key := range dst {
		lcKey := strings.ToLower(key)

		for _, frag := range sensitiveHeaderFragments {
			if strings.Contains(lcKey, frag) {
				dst.Del(key)
				continue nextkey
			}
		}
	}

	return dst
}
