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
	"encoding/json"
	"time"

	"github.com/gogo/protobuf/jsonpb" //nolint:depguard // needed to implement JSONPBUnmarshaler
)

var (
	// json.Marshaler is what both gogo's jsonpb marshal (via json.Marshal) and
	// ghodss/yaml call for fields of this type.
	_ json.Marshaler = DurationStringForJamfSpecV1(0)
	// json.Unmarshaler is called by encoding/json (e.g. via ghodss/yaml in tctl edit).
	_ json.Unmarshaler = (*DurationStringForJamfSpecV1)(nil)
	// jsonpb.JSONPBUnmarshaler is what makes this type's existence worthwhile:
	// gogo's jsonpb checks it before the int64 quote-stripping path that breaks
	// [Duration] roundtripping.
	_ jsonpb.JSONPBUnmarshaler = (*DurationStringForJamfSpecV1)(nil)
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

// MarshalJSON delegates to [Duration.MarshalJSON]. Called by encoding/json,
// which is used by both gogo's jsonpb marshal (via json.Marshal) and
// ghodss/yaml in tctl edit.
func (d DurationStringForJamfSpecV1) MarshalJSON() ([]byte, error) {
	return Duration(d).MarshalJSON()
}

// UnmarshalJSON delegates to [Duration.UnmarshalJSON]. Called by encoding/json
// (e.g. when ghodss/yaml is used by tctl edit).
func (d *DurationStringForJamfSpecV1) UnmarshalJSON(data []byte) error {
	return (*Duration)(d).UnmarshalJSON(data)
}

// UnmarshalJSONPB intercepts gogo's jsonpb unmarshal before it strips quotes
// from the string value of int64 fields, delegating to [Duration.UnmarshalJSON]
// which correctly parses the properly quoted JSON string. We only need this
// special handling during the unmarshaling, because marshaling will consistently
// use [json.Marshaler.MarshalJSON] if available.
func (d *DurationStringForJamfSpecV1) UnmarshalJSONPB(_ *jsonpb.Unmarshaler, data []byte) error {
	return d.UnmarshalJSON(data)
}
