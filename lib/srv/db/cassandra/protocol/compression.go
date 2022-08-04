/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package protocol

import (
	"github.com/datastax/go-cassandra-native-protocol/compression/lz4"
	"github.com/datastax/go-cassandra-native-protocol/compression/snappy"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"github.com/datastax/go-cassandra-native-protocol/segment"
)

// NewBodyCompressor returns a new body compressor for the given compression algorithm
func NewBodyCompressor(c primitive.Compression) frame.BodyCompressor {
	switch c {
	case primitive.CompressionNone:
		return nil
	case primitive.CompressionLz4:
		return &lz4.Compressor{}
	case primitive.CompressionSnappy:
		return &snappy.Compressor{}
	default:
		return nil
	}
}

// NewPayloadCompressor returns a new payload compressor for the given compression algorithm
func NewPayloadCompressor(c primitive.Compression) segment.PayloadCompressor {
	switch c {
	case primitive.CompressionNone:
		return nil
	case primitive.CompressionLz4:
		return &lz4.Compressor{}
	case primitive.CompressionSnappy:
		return nil
	default:
		return nil
	}
}
