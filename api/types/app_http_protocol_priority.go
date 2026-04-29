/*
Copyright 2026 Gravitational, Inc.

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

package types

import (
	"encoding/json"
	"math"

	"github.com/gravitational/trace"
)

const (
	// httpProtocolPriorityHTTP1String is the YAML/JSON string form of
	// HTTPProtocolPriority_HTTP_PROTOCOL_PRIORITY_HTTP1.
	httpProtocolPriorityHTTP1String = "http1"
	// httpProtocolPriorityHTTP2String is the YAML/JSON string form of
	// HTTPProtocolPriority_HTTP_PROTOCOL_PRIORITY_HTTP2.
	httpProtocolPriorityHTTP2String = "http2"
)

// Encode encodes HTTPProtocolPriority in its YAML/JSON string form.
// UNSPECIFIED encodes to the empty string so it round-trips with the
// proto jsontag's omitempty.
func (h *HTTPProtocolPriority) Encode() (string, error) {
	switch *h {
	case HTTPProtocolPriority_HTTP_PROTOCOL_PRIORITY_UNSPECIFIED:
		return "", nil
	case HTTPProtocolPriority_HTTP_PROTOCOL_PRIORITY_HTTP1:
		return httpProtocolPriorityHTTP1String, nil
	case HTTPProtocolPriority_HTTP_PROTOCOL_PRIORITY_HTTP2:
		return httpProtocolPriorityHTTP2String, nil
	default:
		return "", trace.BadParameter("invalid HTTPProtocolPriority value %v", *h)
	}
}

// MarshalJSON marshals HTTPProtocolPriority to its string form.
func (h *HTTPProtocolPriority) MarshalJSON() ([]byte, error) {
	val, err := h.Encode()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := json.Marshal(val)
	return out, trace.Wrap(err)
}

// UnmarshalJSON parses HTTPProtocolPriority from its string form. It
// also accepts numeric proto values for backward compatibility with
// callers that round-trip the raw enum number.
func (h *HTTPProtocolPriority) UnmarshalJSON(data []byte) error {
	var val any
	if err := json.Unmarshal(data, &val); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(h.decode(val))
}

// MarshalYAML marshals HTTPProtocolPriority to its string form.
func (h *HTTPProtocolPriority) MarshalYAML() (interface{}, error) {
	val, err := h.Encode()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return val, nil
}

// UnmarshalYAML parses HTTPProtocolPriority from its string form. It
// also accepts numeric proto values for backward compatibility with
// callers that round-trip the raw enum number.
func (h *HTTPProtocolPriority) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var val any
	if err := unmarshal(&val); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(h.decode(val))
}

func (h *HTTPProtocolPriority) decode(val any) error {
	switch v := val.(type) {
	case string:
		switch v {
		case "":
			*h = HTTPProtocolPriority_HTTP_PROTOCOL_PRIORITY_UNSPECIFIED
		case httpProtocolPriorityHTTP1String:
			*h = HTTPProtocolPriority_HTTP_PROTOCOL_PRIORITY_HTTP1
		case httpProtocolPriorityHTTP2String:
			*h = HTTPProtocolPriority_HTTP_PROTOCOL_PRIORITY_HTTP2
		default:
			return trace.BadParameter("invalid HTTPProtocolPriority value %q", v)
		}
		return nil
	case int32:
		return trace.Wrap(h.setFromEnum(v))
	case int64:
		return trace.Wrap(h.setFromNumeric(float64(v)))
	case int:
		return trace.Wrap(h.setFromNumeric(float64(v)))
	case float64:
		return trace.Wrap(h.setFromNumeric(v))
	case float32:
		return trace.Wrap(h.setFromNumeric(float64(v)))
	case nil:
		*h = HTTPProtocolPriority_HTTP_PROTOCOL_PRIORITY_UNSPECIFIED
		return nil
	default:
		return trace.BadParameter("invalid HTTPProtocolPriority type %T", val)
	}
}

// setFromNumeric validates that v is a whole number in int32 range
// before mapping it to a proto enum value. Without this guard,
// JSON/YAML inputs that decode to a non-integer float (e.g. 1.9) or
// to an out-of-range integer (e.g. 2^33) would silently truncate or
// wrap into a valid enum and apply an unintended ALPN priority.
func (h *HTTPProtocolPriority) setFromNumeric(v float64) error {
	if math.IsNaN(v) || v != math.Trunc(v) {
		return trace.BadParameter("invalid HTTPProtocolPriority numeric value %v: not a whole number", v)
	}
	if v < math.MinInt32 || v > math.MaxInt32 {
		return trace.BadParameter("invalid HTTPProtocolPriority numeric value %v: out of int32 range", v)
	}
	return h.setFromEnum(int32(v))
}

func (h *HTTPProtocolPriority) setFromEnum(val int32) error {
	if _, ok := HTTPProtocolPriority_name[val]; !ok {
		return trace.BadParameter("invalid HTTPProtocolPriority enum %v", val)
	}
	*h = HTTPProtocolPriority(val)
	return nil
}
