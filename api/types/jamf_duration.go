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

// DurationStringForJamfSpecV1 is a transitional type alias for Duration.
//
// It exists so that enterprise call sites can switch to this name before the
// proto casttype for the JamfSpecV1 / JamfInventoryEntry duration fields
// changes in a follow-up PR. Once the casttype is switched, this alias will
// be replaced by a distinct named type that implements jsonpb.JSONPBUnmarshaler
// to work around a serialization bug in gogo's jsonpb.
//
// See https://github.com/gravitational/teleport/issues/57747.
type DurationStringForJamfSpecV1 = Duration
