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

package proxy

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gravitational/trace"
)

// streamFilter filters a Kubernetes list response by reading from src and
// writing filtered output to dst, without buffering the entire response.
type streamFilter interface {
	filter(src io.Reader, dst io.Writer) error
}

// newStreamFilter returns a streaming filter for the given content type,
// or nil if no streaming implementation is available for that type.
func newStreamFilter(contentType string, matcher resourceMatcher) streamFilter {
	switch {
	case strings.Contains(contentType, "application/json"):
		return &jsonStreamFilter{matcher: matcher}
	case strings.Contains(contentType, "application/vnd.kubernetes.protobuf"):
		// Protobuf is a length-prefixed format: every message and repeated field
		// must declare its total byte size before the content. This means we'd
		// need to buffer all filtered items to compute sizes before writing any
		// output, defeating the purpose of streaming. The caller forces
		// Accept: application/json upstream when possible to avoid this path.
		return nil
	default:
		return nil
	}
}

// jsonStreamFilter performs streaming RBAC filtering on a Kubernetes JSON list response.
// It parses the top-level JSON object incrementally using json.Decoder,
// writes non-items fields through unchanged,
// and filters the items/rows array one element at a time using the matcher.
// This avoids buffering the entire response in memory.
//
// Supports both regular list responses ("items" array)
// and server-side Table format ("rows" array with nested object.metadata).
type jsonStreamFilter struct {
	matcher resourceMatcher
}

func (f *jsonStreamFilter) filter(src io.Reader, dst io.Writer) error {
	decoder := json.NewDecoder(src)
	w := &jsonStreamWriter{dst: dst, firstField: true}
	scratch := make(json.RawMessage, 0, 2048)

	// Read opening { of the list object.
	if err := expectDelim(decoder, '{'); err != nil {
		return trace.Wrap(err)
	}
	if err := w.writeRaw("{"); err != nil {
		return trace.Wrap(err)
	}

	for decoder.More() {
		key, err := decodeStringToken(decoder)
		if err != nil {
			return trace.Wrap(err)
		}

		switch key {
		case "items":
			if err := w.writeKey(key); err != nil {
				return trace.Wrap(err)
			}
			scratch, err = f.filterArray(decoder, w, scratch, extractItemMeta)
			if err != nil {
				return trace.Wrap(err)
			}

		case "rows":
			if err := w.writeKey(key); err != nil {
				return trace.Wrap(err)
			}
			scratch, err = f.filterArray(decoder, w, scratch, extractTableRowMeta)
			if err != nil {
				return trace.Wrap(err)
			}

		default:
			scratch = scratch[:0]
			if err := decoder.Decode(&scratch); err != nil {
				return trace.Wrap(err)
			}
			if err := w.writeKeyValue(key, scratch); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	if err := expectDelim(decoder, '}'); err != nil {
		return trace.Wrap(err)
	}
	return w.writeRaw("}")
}

// metaExtractor extracts name and namespace from a JSON item for RBAC matching.
type metaExtractor func(item json.RawMessage) (name, namespace string, err error)

// filterArray reads a JSON array from the decoder, applies RBAC filtering
// to each element, and writes the filtered array to the writer.
// The scratch buffer is reused across calls to reduce allocations.
// Returns the (possibly grown) scratch buffer for reuse.
func (f *jsonStreamFilter) filterArray(
	decoder *json.Decoder,
	w *jsonStreamWriter,
	scratch json.RawMessage,
	extract metaExtractor,
) (json.RawMessage, error) {
	// Check if the next token is null (items: null).
	token, err := decoder.Token()
	if err != nil {
		return scratch, trace.Wrap(err)
	}
	if token == nil {
		return scratch, w.writeRaw("null")
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '[' {
		return scratch, trace.BadParameter("expected [ or null, got %v", token)
	}

	if err := w.writeRaw("["); err != nil {
		return scratch, trace.Wrap(err)
	}

	firstItem := true
	for decoder.More() {
		scratch = scratch[:0]
		if err := decoder.Decode(&scratch); err != nil {
			return scratch, trace.Wrap(err)
		}

		name, namespace, err := extract(scratch)
		if err != nil {
			// Can't extract metadata — fail closed (deny).
			continue
		}

		allowed, err := f.matcher.match(name, namespace)
		if err != nil {
			return scratch, trace.Wrap(err)
		}
		if !allowed {
			continue
		}

		if !firstItem {
			if err := w.writeRaw(","); err != nil {
				return scratch, trace.Wrap(err)
			}
		}
		firstItem = false

		if _, err := w.dst.Write(scratch); err != nil {
			return scratch, trace.Wrap(err)
		}
	}

	if err := expectDelim(decoder, ']'); err != nil {
		return scratch, trace.Wrap(err)
	}
	return scratch, w.writeRaw("]")
}

// kubeItemEnvelope is the minimal struct needed to extract name and namespace
// from a regular Kubernetes list item for RBAC filtering.
type kubeItemEnvelope struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
}

// kubeTableRowEnvelope is the minimal struct needed to extract name and namespace
// from a Kubernetes Table row for RBAC filtering.
type kubeTableRowEnvelope struct {
	Object struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
	} `json:"object"`
}

// extractItemMeta extracts name and namespace from a regular list item.
func extractItemMeta(item json.RawMessage) (name, namespace string, err error) {
	var env kubeItemEnvelope
	if err := json.Unmarshal(item, &env); err != nil {
		return "", "", trace.Wrap(err)
	}
	return env.Metadata.Name, env.Metadata.Namespace, nil
}

// extractTableRowMeta extracts name and namespace from a Table row.
func extractTableRowMeta(item json.RawMessage) (name, namespace string, err error) {
	var env kubeTableRowEnvelope
	if err := json.Unmarshal(item, &env); err != nil {
		return "", "", trace.Wrap(err)
	}
	return env.Object.Metadata.Name, env.Object.Metadata.Namespace, nil
}

// jsonStreamWriter handles JSON output formatting with comma tracking.
type jsonStreamWriter struct {
	dst        io.Writer
	firstField bool // tracks whether the next top-level field is the first
}

// writeRaw writes a raw string to the output.
func (w *jsonStreamWriter) writeRaw(s string) error {
	_, err := io.WriteString(w.dst, s)
	return trace.Wrap(err)
}

// writeKey writes a JSON key with comma separator if needed.
func (w *jsonStreamWriter) writeKey(key string) error {
	if !w.firstField {
		if err := w.writeRaw(","); err != nil {
			return trace.Wrap(err)
		}
	}
	w.firstField = false

	keyBytes, err := json.Marshal(key)
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := w.dst.Write(keyBytes); err != nil {
		return trace.Wrap(err)
	}
	return w.writeRaw(":")
}

// writeKeyValue writes a JSON key-value pair.
func (w *jsonStreamWriter) writeKeyValue(key string, value json.RawMessage) error {
	if err := w.writeKey(key); err != nil {
		return trace.Wrap(err)
	}
	_, err := w.dst.Write(value)
	return trace.Wrap(err)
}

// expectDelim reads the next token from the decoder and verifies it matches the expected delimiter.
func expectDelim(decoder *json.Decoder, expected rune) error {
	token, err := decoder.Token()
	if err != nil {
		return trace.Wrap(err)
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != json.Delim(expected) {
		return trace.BadParameter("expected %c, got %v", expected, token)
	}
	return nil
}

// decodeStringToken reads the next token from the decoder and returns it as a string.
func decodeStringToken(decoder *json.Decoder) (string, error) {
	token, err := decoder.Token()
	if err != nil {
		return "", trace.Wrap(err)
	}
	s, ok := token.(string)
	if !ok {
		return "", trace.BadParameter("expected string key, got %v", token)
	}
	return s, nil
}

// wrapContentEncoding returns a reader/writer pair that handles Content-Encoding.
// For gzip, it wraps the reader in a gzip decompressor and the writer in a pooled gzip compressor.
// For identity or no encoding, it returns no-op closers.
// Returns an error for unsupported encodings.
func wrapContentEncoding(r io.Reader, w io.Writer, contentEncoding string) (io.ReadCloser, io.WriteCloser, error) {
	switch contentEncoding {
	case "", "identity":
		return io.NopCloser(r), &nopCloserWrapper{w}, nil
	case "gzip":
		gzReader, err := gzip.NewReader(r)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		gzWriter := gzipPool.Get().(*gzip.Writer)
		gzWriter.Reset(w)
		return gzReader, &gzipWrapper{gzWriter}, nil
	default:
		return nil, nil, fmt.Errorf("unsupported Content-Encoding: %s", contentEncoding)
	}
}

// headerCapturer is an http.ResponseWriter that captures headers and status,
// then streams the body to an io.Writer.
type headerCapturer struct {
	body        io.Writer
	headers     http.Header
	status      int
	once        sync.Once
	wroteHeader chan struct{} // closed once headers are captured; used as a signal
}

func newHeaderCapturer(body io.Writer) *headerCapturer {
	return &headerCapturer{
		body:        body,
		headers:     make(http.Header),
		wroteHeader: make(chan struct{}),
	}
}

func (h *headerCapturer) Header() http.Header {
	return h.headers
}

func (h *headerCapturer) WriteHeader(statusCode int) {
	h.once.Do(func() {
		h.status = statusCode
		close(h.wroteHeader)
	})
}

func (h *headerCapturer) Write(b []byte) (int, error) {
	h.WriteHeader(http.StatusOK)
	return h.body.Write(b)
}
