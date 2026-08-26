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

package responsewriters

import (
	"io"
	"mime"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
)

// FilterObj is the interface a Kubernetes Resource Object filter must implement.
type FilterObj interface {
	// FilterObj receives a runtime.Object type and filters the resources on it
	// based on allowed and denied rules.
	// After filtering them, the obj is manipulated to hold the filtered information.
	// The isAllowed boolean returned indicates if the client is allowed to receive the event
	// with the object.
	// The isListObj boolean returned indicates if the object is a list of resources.
	FilterObj(obj runtime.Object) (isAllowed bool, isListObj bool, err error)
}

// FilterBuffer is the interface a Kubernetes Resource response filter must implement.
type FilterBuffer interface {
	// FilterBuffer receives a byte array, decodes the response into the appropriate
	// type and filters the resources based on allowed and denied rules configured.
	// After filtering them, it serializes the response and dumps it into output buffer.
	// If any error occurs, the call returns an error.
	FilterBuffer(buf []byte, output io.Writer) error
}

// Filter is the interface a Kubernetes Resource filter must implement.
type Filter interface {
	FilterObj
	FilterBuffer
}

// FilterWrapper is the wrapper function signature that once executed creates
// a runtime filter for Pods.
// The filter exclusion criteria is:
// - deniedPods: excluded if (namespace,name) matches an entry even if it matches
// the allowedPod's list.
// - allowedPods: excluded if (namespace,name) not match a single entry.
type FilterWrapper func(contentType string, responseCode int) (Filter, error)

// GetContentTypeHeader checks for the presence of the "Content-Type" header and
// returns its value or returns the default content-type: "application/json".
func GetContentTypeHeader(header http.Header) string {
	contentType := header.Get(ContentTypeHeader)
	if len(contentType) > 0 {
		return contentType
	}
	return DefaultContentType
}

// SetContentTypeHeader checks for the presence of the "Content-Type" header and
// sets its media type value or sets the default content-type: "application/json".
func SetContentTypeHeader(w http.ResponseWriter, header http.Header) {
	contentType := header.Get(ContentTypeHeader)
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		w.Header().Set(ContentTypeHeader, mediaType)
		return
	}
	w.Header().Set(ContentTypeHeader, DefaultContentType)
}
