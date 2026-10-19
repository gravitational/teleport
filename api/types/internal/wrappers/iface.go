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

package wrappers

// GogoprotoCustomType is the interface that must be satisfied for a type to be
// used as the target of the gogoproto.customtype field option. The contract of
// the custom type is described in
// https://github.com/gogo/protobuf/blob/master/custom_types.md but it's not
// defined in any Go interface type in gogoproto itself. It's only defined here
// as a self-documenting check for existing custom types in use, but no new
// custom types should be added.
type GogoprotoCustomType interface {
	Size() int
	Marshal() ([]byte, error)
	MarshalTo(data []byte) (int, error)
	Unmarshal(data []byte) error
}
