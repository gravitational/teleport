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
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/gravitational/teleport/api/types"
)

// convertStructPB converts a structpb.Struct to a gogo-compatible
// `types.Struct`. Inspired by `apievents.Resource153ToStruct()`] but avoiding a
// potentially heavyweight import, and skipping an unnecessary jsonpb
// conversion.
func convertStructPB(s *structpb.Struct) (*types.Struct, error) {
	encoded, err := proto.Marshal(s)
	if err != nil {
		return nil, trace.Wrap(err, "marshaling proto struct")
	}

	// unmarshal directly into the wrapper's inner type to avoid a json
	// conversion.
	out := &types.Struct{}
	if err := out.Struct.Unmarshal(encoded); err != nil {
		return nil, trace.Wrap(err, "unmarshaling into gogo struct")
	}

	return out, nil
}
