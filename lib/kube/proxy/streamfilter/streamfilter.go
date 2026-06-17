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

// Package streamfilter implements streaming RBAC filtering for Kubernetes
// list responses. It uses json.Decoder with json.RawMessage to decode
// individual items incrementally, apply a matcher to each item, and write
// matching items to the output without buffering the entire response.
package streamfilter

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"

	"github.com/gravitational/trace"
)

// Matcher matches a Kubernetes resource by name and namespace.
type Matcher interface {
	Match(name, namespace string) (bool, error)
}

// Filter filters a Kubernetes list response by reading from src and
// writing filtered output to dst, without buffering the entire response.
type Filter interface {
	Filter(src io.Reader, dst io.Writer) error
}

// NewJSONFilter returns a streaming filter for Kubernetes JSON list responses.
// Callers are responsible for ensuring the upstream response is JSON
// (e.g. by forcing Accept: application/json on the upstream request).
// If log is nil, slog.Default() is used.
func NewJSONFilter(matcher Matcher, log *slog.Logger) *JSONFilter {
	if log == nil {
		log = slog.Default()
	}
	return &JSONFilter{matcher: matcher, log: log}
}

// JSONFilter performs streaming RBAC filtering on a Kubernetes JSON list response.
// It parses the top-level JSON object incrementally using json.Decoder,
// writes non-items fields through unchanged,
// and filters the items/rows array one element at a time using the matcher.
// This avoids buffering the entire response in memory.
//
// Supports both regular list responses ("items" array)
// and server-side Table format ("rows" array with nested object.metadata).
type JSONFilter struct {
	matcher Matcher
	log     *slog.Logger
}

func (f *JSONFilter) Filter(src io.Reader, dst io.Writer) error {
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
func (f *JSONFilter) filterArray(
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
			f.log.WarnContext(context.Background(), "Unable to extract name/namespace from list item", "error", err)
			continue
		}

		allowed, err := f.matcher.Match(name, namespace)
		if err != nil {
			f.log.WarnContext(context.Background(), "Unable to compile regex expressions within kubernetes_resources", "error", err)
			continue
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
	return scratch, trace.Wrap(w.writeRaw("]"))
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
	Object *struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
	} `json:"object"`
}

func extractItemMeta(item json.RawMessage) (name, namespace string, err error) {
	var env kubeItemEnvelope
	if err := json.Unmarshal(item, &env); err != nil {
		return "", "", trace.Wrap(err)
	}
	if env.Metadata.Name == "" {
		return "", "", trace.BadParameter("item has no name")
	}
	return env.Metadata.Name, env.Metadata.Namespace, nil
}

func extractTableRowMeta(item json.RawMessage) (name, namespace string, err error) {
	var env kubeTableRowEnvelope
	if err := json.Unmarshal(item, &env); err != nil {
		return "", "", trace.Wrap(err)
	}
	if env.Object == nil {
		return "", "", trace.BadParameter("table row has no embedded object")
	}
	if env.Object.Metadata.Name == "" {
		return "", "", trace.BadParameter("table row embedded object has no name")
	}
	return env.Object.Metadata.Name, env.Object.Metadata.Namespace, nil
}

// jsonStreamWriter handles JSON output formatting with comma tracking.
type jsonStreamWriter struct {
	dst        io.Writer
	firstField bool // tracks whether the next top-level field is the first
}

func (w *jsonStreamWriter) writeRaw(s string) error {
	_, err := io.WriteString(w.dst, s)
	return trace.Wrap(err)
}

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
	return trace.Wrap(w.writeRaw(":"))
}

func (w *jsonStreamWriter) writeKeyValue(key string, value json.RawMessage) error {
	if err := w.writeKey(key); err != nil {
		return trace.Wrap(err)
	}
	_, err := w.dst.Write(value)
	return trace.Wrap(err)
}

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
