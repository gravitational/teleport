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
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// TestHTTPProtocolPriorityDecode covers the YAML / JSON decode paths
// for HTTPProtocolPriority. The numeric cases pin the rejection
// behavior for non-integer floats and out-of-range integers; without
// these checks the cast-to-int32 in the decoder silently truncates
// fractional values and wraps overflow values into a valid enum.
func TestHTTPProtocolPriorityDecode(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    HTTPProtocolPriority
		wantErr bool
	}{
		{name: "string http1", input: `"http1"`, want: HTTPProtocolPriority_HTTP_PROTOCOL_PRIORITY_HTTP1},
		{name: "string http2", input: `"http2"`, want: HTTPProtocolPriority_HTTP_PROTOCOL_PRIORITY_HTTP2},
		{name: "empty string", input: `""`, want: HTTPProtocolPriority_HTTP_PROTOCOL_PRIORITY_UNSPECIFIED},
		{name: "string alias rejected", input: `"prefer-h2"`, wantErr: true},
		{name: "numeric 0", input: `0`, want: HTTPProtocolPriority_HTTP_PROTOCOL_PRIORITY_UNSPECIFIED},
		{name: "numeric 2", input: `2`, want: HTTPProtocolPriority_HTTP_PROTOCOL_PRIORITY_HTTP2},
		{name: "numeric 99 rejected", input: `99`, wantErr: true},
		{name: "fractional float rejected", input: `1.9`, wantErr: true},
		{name: "negative float rejected", input: `-1.5`, wantErr: true},
		{name: "out-of-range int rejected", input: `4294967298`, wantErr: true},
	}
	for _, tc := range cases {
		t.Run("json/"+tc.name, func(t *testing.T) {
			var got HTTPProtocolPriority
			err := json.Unmarshal([]byte(tc.input), &got)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
		t.Run("yaml/"+tc.name, func(t *testing.T) {
			var got HTTPProtocolPriority
			err := yaml.Unmarshal([]byte(tc.input), &got)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestHTTPProtocolPriorityEncode pins the string form emitted on
// MarshalJSON / MarshalYAML for each enum value, so callers
// (`tctl get`, `kubectl get appv3`, terraform state) see stable
// round-trip output.
func TestHTTPProtocolPriorityEncode(t *testing.T) {
	cases := []struct {
		name string
		in   HTTPProtocolPriority
		want string
	}{
		{name: "unspecified", in: HTTPProtocolPriority_HTTP_PROTOCOL_PRIORITY_UNSPECIFIED, want: ""},
		{name: "http1", in: HTTPProtocolPriority_HTTP_PROTOCOL_PRIORITY_HTTP1, want: "http1"},
		{name: "http2", in: HTTPProtocolPriority_HTTP_PROTOCOL_PRIORITY_HTTP2, want: "http2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.in.Encode()
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestHTTPProtocolPriorityNaNRejected covers the explicit NaN
// rejection branch in setFromNumeric, which json.Unmarshal does not
// surface (JSON has no NaN literal), but YAML and direct callers
// can produce.
func TestHTTPProtocolPriorityNaNRejected(t *testing.T) {
	var h HTTPProtocolPriority
	require.Error(t, h.decode(math.NaN()))
}
