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
	"math"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
)

// MaxEncodedDelegationBytes is the maximum size, in bytes, a string accepted by
// Decode may be. This limit is used to prevent DoS attacks by unauthenticated
// clients, during certificate validation.
func MaxEncodedDelegationBytes() int {
	return int(math.Ceil(maxBytesPerLink * MaxChainLength * base64Overhead))
}

const (
	// We estimate a maximum of roughly 4KiB for each link in the delegation chain.
	maxBytesPerLink = 4096

	// On average, base64 encoding increases the size of the output by ~33%.
	base64Overhead = 1.33
)

// Encode returns a compact encoding of a delegation for use in certificates and
// other space-constrained contexts.
func Encode(delegation *delegationv1.Delegation) (string, error) {
	bytes, err := proto.Marshal(delegation)
	if err != nil {
		return "", trace.Wrap(err)
	}
	str := base64.RawURLEncoding.EncodeToString(bytes)
	if len(str) > MaxEncodedDelegationBytes() {
		return "", trace.LimitExceeded("encoded delegation is too large")
	}
	return str, nil
}

// Decode a delegator from a string previously encoded with Encode.
func Decode(str string) (*delegationv1.Delegation, error) {
	if str == "" {
		return nil, nil
	}
	if len(str) > MaxEncodedDelegationBytes() {
		return nil, trace.LimitExceeded("encoded delegation is too large")
	}
	bytes, err := base64.RawURLEncoding.DecodeString(str)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var delegation delegationv1.Delegation
	if err := proto.Unmarshal(bytes, &delegation); err != nil {
		return nil, trace.Wrap(err)
	}
	return &delegation, nil
}
