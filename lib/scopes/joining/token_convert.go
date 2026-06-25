// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package joining

import (
	"bytes"

	"github.com/gogo/protobuf/jsonpb" //nolint:depguard // needed for backwards compatibility with gogoproto-generated types.Struct
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/gravitational/teleport/api/types"
)

// convertStructPB converts a structpb.Struct to a gogo-compatible
// `types.Struct`. Inspired by `apievents.Resource153ToStruct()`] but avoiding a
// potentially heavyweight import.
func convertStructPB(s *structpb.Struct) (*types.Struct, error) {
	encoded, err := protojson.Marshal(s)
	if err != nil {
		return nil, trace.Wrap(err, "marshaling protojson")
	}

	out := &types.Struct{}
	if err = (&jsonpb.Unmarshaler{
		AllowUnknownFields: true,
	}).Unmarshal(bytes.NewReader(encoded), out); err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}
