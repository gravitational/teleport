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

package delegation

import (
	"encoding/base64"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/trace"
)

// Encode returns a compact encoding of a delegation for use in certificates and
// other space-constrained contexts.
func Encode(delegation *types.Delegation) (string, error) {
	if delegation == nil {
		return "", nil
	}
	bytes, err := delegation.Marshal()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// Decode a delegator from a string previously encoded with Encode.
func Decode(str string) (*types.Delegation, error) {
	if str == "" {
		return nil, nil
	}
	bytes, err := base64.RawURLEncoding.DecodeString(str)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var delegator types.Delegation
	if err := delegator.Unmarshal(bytes); err != nil {
		return nil, trace.Wrap(err)
	}
	return &delegator, nil
}
