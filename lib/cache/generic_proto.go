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

package cache

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

func newCachedGogoResource[M gogoMessage[S], S any](key string, rsc M) (*cachedGogoResource[M, S], error) {
	wire, err := rsc.Marshal()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// check that we can successfully unmarshal the data in wire, so we can be
	// reasonably sure that cloneResource won't panic
	if err := M(new(S)).Unmarshal(wire); err != nil {
		return nil, trace.Wrap(err)
	}
	return &cachedGogoResource[M, S]{
		key:  key,
		rsc:  rsc,
		wire: string(wire),
	}, nil
}

// cachedGogoResource is a resource in cache represented by a gogoproto message
// that we also keep in its marshaled protobuf form.
type cachedGogoResource[M gogoMessage[S], S any] struct {
	key  string
	rsc  M
	wire string
}

// cloneResource unmarshals the resource into a new message, owned by the
// caller.
func (c *cachedGogoResource[M, S]) cloneResource() M {
	res := M(new(S))
	if err := res.Unmarshal([]byte(c.wire)); err != nil {
		// wire is supposed to always be unmarshable without errors into a M,
		// and we check this in [newCachedGogoResource]; cloning a message via
		// protoadapt (i.e. [utils.CloneProtoMsg]) will panic in case of an
		// error during marshal or unmarshal, so this is strictly more reliable
		//
		// TODO(okraport): instrument this with an Unreachable assertion
		panic(err)
	}
	return res
}

// clone returns a new owned copy of the cached resource.
func (c *cachedGogoResource[M, S]) clone() *cachedGogoResource[M, S] {
	return &cachedGogoResource[M, S]{
		key:  c.key,
		rsc:  c.cloneResource(),
		wire: c.wire,
	}
}
