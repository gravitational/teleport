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

package utils

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

// GetAndReplaceRequestBody returns the request body and replaces the drained
// body reader with io.NopCloser allowing for further body processing by http
// transport.
func GetAndReplaceRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil || req.Body == http.NoBody {
		return []byte{}, nil
	}
	// req.Body is closed during tryDrainBody call.
	payload, err := tryDrainBody(req.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Replace the drained body with io.NopCloser reader allowing for further request processing by HTTP transport.
	req.Body = io.NopCloser(bytes.NewReader(payload))
	return payload, nil
}

// GetAndReplaceResponseBody returns the response body and replaces the drained
// body reader with io.NopCloser allowing for further body processing.
func GetAndReplaceResponseBody(response *http.Response) ([]byte, error) {
	if response.Body == nil {
		return []byte{}, nil
	}

	payload, err := tryDrainBody(response.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response.Body = io.NopCloser(bytes.NewReader(payload))
	return payload, nil
}

// ReplaceRequestBody drains the old request body and replaces it with a new one.
func ReplaceRequestBody(req *http.Request, newBody io.ReadCloser) error {
	if _, err := tryDrainBody(req.Body); err != nil {
		return trace.Wrap(err)
	}
	req.Body = newBody
	return nil
}

// tryDrainBody tries to drain and close the body, returning the read bytes.
// It may fail to completely drain the body if the size of the body exceeds MaxHTTPRequestSize.
func tryDrainBody(b io.ReadCloser) (payload []byte, err error) {
	if b == nil {
		return nil, nil
	}
	defer func() {
		if closeErr := b.Close(); closeErr != nil {
			err = trace.NewAggregate(err, closeErr)
		}
	}()
	payload, err = ReadAtMost(b, teleport.MaxHTTPRequestSize)
	if err != nil {
		err = trace.Wrap(err)
		return
	}
	return
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
