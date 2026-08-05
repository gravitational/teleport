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

// Package gogofriendlyprotocodec overrides the default protobuf codec defined
// in grpc-go with one that supports gogoproto messages more efficiently.
// Importing this package will register the codec.
package gogofriendlyprotocodec

import (
	gogoproto "github.com/gogo/protobuf/proto"
	"google.golang.org/grpc/encoding"
	_ "google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/mem"

	"github.com/gravitational/teleport/api/types"
)

var grpcProtoCodecV2 encoding.CodecV2

func init() {
	// we need to wrap the default proto codec at init time, and by importing
	// the standard codec package we guarantee that its init (and thus its codec
	// registration) runs before ours

	grpcProtoCodecV2 = encoding.GetCodecV2("proto")
	if grpcProtoCodecV2 == nil {
		panic("grpc-go proto codec missing")
	}

	encoding.RegisterCodecV2(codecV2{})
}

type codecV2 struct{}

func (codecV2) Name() string {
	return "proto"
}

type gogoMarshaler = interface {
	gogoproto.Message
	gogoproto.Marshaler
}

func (codecV2) Marshal(v any) (data mem.BufferSlice, err error) {
	if gogomsg, ok := v.(gogoMarshaler); ok {
		buf, err := gogomsg.Marshal()
		if err != nil {
			return nil, err
		}
		return mem.BufferSlice{mem.SliceBuffer(buf)}, nil
	}

	return grpcProtoCodecV2.Marshal(v)
}

type gogoUnmarshaler = interface {
	gogoproto.Message
	gogoproto.Unmarshaler
}

var _ gogoUnmarshaler = (*types.RoleV6)(nil)

func (codecV2) Unmarshal(data mem.BufferSlice, v any) (err error) {
	if gogomsg, ok := v.(gogoUnmarshaler); ok {
		buf := data.MaterializeToBuffer(mem.DefaultBufferPool())
		defer buf.Free()
		return gogomsg.Unmarshal(buf.ReadOnlyData())
	}

	return grpcProtoCodecV2.Unmarshal(data, v)
}
