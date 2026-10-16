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

package internal

import (
	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"
)

type gogoMessage[S any] = interface {
	gogoproto.Message
	gogoproto.Marshaler
	gogoproto.Unmarshaler

	*S
}

// GogoMessageDriver implements marshaling and unmarshaling of a [Driver] for a
// resource backed by a gogoproto message.
type GogoMessageDriver[M gogoMessage[S], S any] struct{}

// Marshal implements [Driver].
func (GogoMessageDriver[M, S]) Marshal(message *S) (string, error) {
	wire, err := M(message).Marshal()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(wire), nil
}

// Unmarshal implements [Driver].
func (GogoMessageDriver[M, S]) Unmarshal(wire string) (M, error) {
	message := M(new(S))
	if err := message.Unmarshal([]byte(wire)); err != nil {
		return nil, trace.Wrap(err)
	}
	return message, nil
}
