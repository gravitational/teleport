// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"time"

	"github.com/gogo/protobuf/jsonpb" //nolint:depguard // needed to implement JSONPBUnmarshaler
	"github.com/gravitational/trace"
)

var (
	// jsonpb.JSONPBUnmarshaler is what makes this type's existence worthwhile:
	// gogo's jsonpb checks it before the int64 quote-stripping path that breaks
	// [Duration] roundtripping.
	_ jsonpb.JSONPBUnmarshaler = (*DurationStringForJamfSpecV1)(nil)
	_ jsonpb.JSONPBMarshaler   = (*DurationStringForJamfSpecV1)(nil)
)

// DurationStringForJamfSpecV1 is a [Duration]-like casttype used only by the
// JamfSpecV1 and JamfInventoryEntry proto fields. It exists to work around a
// bug in gogoproto's jsonpb where int64 casttype fields with a custom
// MarshalJSON can't be roundtripped: marshal produces a quoted duration
// string, but unmarshal strips the quotes for int64 fields before passing the
// value to encoding/json, which then fails because the bare duration string
// (e.g. `6h0m0s`) is not valid JSON.
//
// Implementing JSONPBUnmarshaler bypasses the broken path because gogo's
// jsonpb checks that interface before the int64 quote-stripping.
//
// See https://github.com/gravitational/teleport/issues/57747.
type DurationStringForJamfSpecV1 time.Duration

// MarshalJSON implements [json.Marshaler] as an unconditional error.
func (d DurationStringForJamfSpecV1) MarshalJSON() ([]byte, error) {
	return nil, trace.Errorf("DurationStringForJamfSpecV1 should not be marshaled with encoding/json directly")
}

// UnmarshalJSON implements [json.Unmarshaler] as an unconditional error.
func (d *DurationStringForJamfSpecV1) UnmarshalJSON(data []byte) error {
	return trace.Errorf("DurationStringForJamfSpecV1 should not be unmarshaled with encoding/json directly")
}

// MarshalJSONPB implements [jsonpb.JSONPBMarshaler].
func (d DurationStringForJamfSpecV1) MarshalJSONPB(*jsonpb.Marshaler) ([]byte, error) {
	return Duration(d).MarshalJSON()
}

// UnmarshalJSONPB implements [jsonpb.JSONPBUnmarshaler].
func (d *DurationStringForJamfSpecV1) UnmarshalJSONPB(_ *jsonpb.Unmarshaler, data []byte) error {
	return (*Duration)(d).UnmarshalJSON(data)
}
