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
	"encoding/json"
	"io"

	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/gravitational/teleport/api/types"
)

// streamingJSONFilter filters a Kubernetes list response by streaming through
// the JSON without buffering the entire response in memory.
//
// It works by:
// 1. Parsing the JSON structure incrementally using a streaming decoder
// 2. Writing the envelope (apiVersion, kind, metadata) immediately
// 3. Processing items[] array one item at a time
// 4. Filtering each item and writing allowed items immediately
// 5. Closing the JSON structure
//
// This significantly reduces memory usage for large list responses and allows
// for faster initial response times since items can be sent to the client
// as they are processed.
type streamingJSONFilter struct {
	src              io.Reader
	dst              io.Writer
	metaResource     metaResource
	allowedResources []types.KubernetesResource
	deniedResources  []types.KubernetesResource
	itemsProcessed   int
	itemsFiltered    int
	firstField       bool
}

// newStreamingJSONFilter creates a new streaming JSON filter.
func newStreamingJSONFilter(
	src io.Reader,
	dst io.Writer,
	metaResource metaResource,
	allowedResources []types.KubernetesResource,
	deniedResources []types.KubernetesResource,
) *streamingJSONFilter {
	return &streamingJSONFilter{
		src:              src,
		dst:              dst,
		metaResource:     metaResource,
		allowedResources: allowedResources,
		deniedResources:  deniedResources,
		firstField:       true,
	}
}

// writeCommaIfNotFirst writes a comma separator if this isn't the first field.
func (s *streamingJSONFilter) writeCommaIfNotFirst() error {
	if !s.firstField {
		if err := s.writeComma(); err != nil {
			return trace.Wrap(err)
		}
	}
	s.firstField = false
	return nil
}

func (s *streamingJSONFilter) writeComma() error {
	_, err := io.WriteString(s.dst, ",")
	return trace.Wrap(err)
}

func (s *streamingJSONFilter) writeOpeningBrace() error {
	_, err := io.WriteString(s.dst, "{")
	return trace.Wrap(err)
}

func (s *streamingJSONFilter) writeClosingBrace() error {
	_, err := io.WriteString(s.dst, "}")
	return trace.Wrap(err)
}

func (s *streamingJSONFilter) writeOpeningBracket() error {
	_, err := io.WriteString(s.dst, "[")
	return trace.Wrap(err)
}

func (s *streamingJSONFilter) writeClosingBracket() error {
	_, err := io.WriteString(s.dst, "]")
	return trace.Wrap(err)
}

func (s *streamingJSONFilter) writeColon() error {
	_, err := io.WriteString(s.dst, ":")
	return trace.Wrap(err)
}

func (s *streamingJSONFilter) writeNull() error {
	_, err := io.WriteString(s.dst, "null")
	return trace.Wrap(err)
}

// writeKey writes a JSON key with proper formatting and comma handling.
func (s *streamingJSONFilter) writeKey(key string) error {
	if err := s.writeCommaIfNotFirst(); err != nil {
		return trace.Wrap(err)
	}

	// Marshal the key to get properly quoted JSON string without trailing newline
	keyBytes, err := json.Marshal(key)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := s.dst.Write(keyBytes); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(s.writeColon())
}

func (s *streamingJSONFilter) writeEntry(key string, value json.RawMessage) error {
	// Write key
	if err := s.writeKey(key); err != nil {
		return trace.Wrap(err)
	}

	// Write value directly (json.RawMessage is already valid JSON)
	if _, err := s.dst.Write(value); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// filter performs the streaming filtering operation.
func (s *streamingJSONFilter) filter() error {
	// Use json.Decoder for streaming JSON parsing
	decoder := json.NewDecoder(s.src)

	scratchRawJson := make(json.RawMessage, 0, 2048)

	// Read the opening brace of the list object
	token, err := decoder.Token()
	if err != nil {
		return trace.Wrap(err)
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return trace.BadParameter("expected JSON object, got %v", token)
	}

	// Write opening brace
	if err := s.writeOpeningBrace(); err != nil {
		return trace.Wrap(err)
	}

	// Iterate through the JSON object fields
	for decoder.More() {
		// Read field name
		token, err := decoder.Token()
		if err != nil {
			return trace.Wrap(err)
		}

		key, ok := token.(string)
		if !ok {
			return trace.BadParameter("expected string key, got %v", token)
		}

		switch key {
		case "items":
			// Write the "items" key using consistent method
			if err := s.writeKey(key); err != nil {
				return trace.Wrap(err)
			}

			// Process the items array with filtering
			if err := s.filterItemsArray(decoder, scratchRawJson); err != nil {
				return trace.Wrap(err)
			}

		default:
			// Decode and write non-items fields
			scratchRawJson = scratchRawJson[:0]
			if err := decoder.Decode(&scratchRawJson); err != nil {
				return trace.Wrap(err)
			}
			if err := s.writeEntry(key, scratchRawJson); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	// Read closing brace
	token, err = decoder.Token()
	if err != nil {
		return trace.Wrap(err)
	}
	delim, ok = token.(json.Delim)
	if !ok || delim != '}' {
		return trace.BadParameter("expected }, got %v", token)
	}

	// Write closing brace
	return s.writeClosingBrace()
}

// filterItemsArray processes the items array, filtering each item.
func (s *streamingJSONFilter) filterItemsArray(decoder *json.Decoder, scratchRawJson json.RawMessage) error {
	// Read and validate opening bracket
	token, err := decoder.Token()
	if err != nil {
		return trace.Wrap(err)
	}

	if token == nil {
		// Handle null items array
		return trace.Wrap(s.writeNull())
	}

	if delim, ok := token.(json.Delim); !ok || delim != '[' {
		return trace.BadParameter("expected array, got %v", token)
	}

	// Write opening bracket
	if err := s.writeOpeningBracket(); err != nil {
		return trace.Wrap(err)
	}

	firstItem := true

	// Stream and filter each item in the array
	for decoder.More() {
		// Decode one item at a time (streaming approach)
		if err := decoder.Decode(&scratchRawJson); err != nil {
			return trace.Wrap(err)
		}

		s.itemsProcessed++

		// Apply RBAC filtering
		if !s.shouldIncludeItem(scratchRawJson) {
			s.itemsFiltered++
			continue
		}

		// Write comma separator between items
		if !firstItem {
			if err := s.writeComma(); err != nil {
				return trace.Wrap(err)
			}
		}
		firstItem = false

		// Write the allowed item directly (json.RawMessage is already valid JSON)
		if _, err := s.dst.Write(scratchRawJson); err != nil {
			return trace.Wrap(err)
		}
	}

	// Read and validate closing bracket
	token, err = decoder.Token()
	if err != nil {
		return trace.Wrap(err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != ']' {
		return trace.BadParameter("expected ], got %v", token)
	}

	// Write closing bracket
	return s.writeClosingBracket()
}

// shouldIncludeItem determines if an item should be included based on RBAC rules.
// Returns false on any error to fail closed (deny by default).
func (s *streamingJSONFilter) shouldIncludeItem(item json.RawMessage) bool {
	// Parse item envelope to extract metadata
	var envelope kubeEnvelope
	if err := json.Unmarshal(item, &envelope); err != nil {
		// Parsing errors mean we can't verify RBAC - deny by default
		return false
	}

	// Build resource descriptor for RBAC matching
	gvk := envelope.GroupVersionKind()
	resource := getKubeResource(
		s.metaResource.requestedResource.resourceKind,
		gvk.Group,
		s.metaResource.verb,
		envelope,
	)

	// Check if resource matches RBAC rules
	allowed, err := matchKubernetesResource(
		resource,
		s.metaResource.isClusterWideResource(),
		s.allowedResources,
		s.deniedResources,
	)
	if err != nil {
		// RBAC matching errors - deny by default
		return false
	}

	return allowed
}

// kubeEnvelope represents the minimal fields needed from a Kubernetes resource
// for RBAC filtering (apiVersion, kind, metadata).
type kubeEnvelope struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Metadata   metav1.ObjectMeta `json:"metadata"`
}

// GroupVersionKind extracts the GVK from the envelope's apiVersion and kind.
func (k kubeEnvelope) GroupVersionKind() schema.GroupVersionKind {
	gv, err := schema.ParseGroupVersion(k.APIVersion)
	if err != nil {
		// Return empty GVK on parse error
		return schema.GroupVersionKind{}
	}
	return gv.WithKind(k.Kind)
}

// GetNamespace returns the namespace from metadata.
func (k kubeEnvelope) GetNamespace() string {
	return k.Metadata.Namespace
}

// GetName returns the resource name from metadata.
func (k kubeEnvelope) GetName() string {
	return k.Metadata.Name
}

type rawJSONKeyword json.Delim

func (r rawJSONKeyword) MarshalText() ([]byte, error) {
	return []byte(json.Delim(r).String()), nil
}
