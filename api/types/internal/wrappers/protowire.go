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

import (
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protowire"
)

// The tags and tag sizes of a field with field number 1 or 2 and type bytes
// (i.e. string, bytes or a message). Checked against the output of the
// appropriate protowire functions in TestConsts.
const (
	fieldOneTypeBytesTag    = uint64(protowire.Number(1)<<3) | uint64(protowire.BytesType&7)
	fieldOneTagSize         = 1
	fieldOneTypeBytesVarint = byte(fieldOneTypeBytesTag)

	fieldTwoTypeBytesTag    = uint64(protowire.Number(2)<<3) | uint64(protowire.BytesType&7)
	fieldTwoTagSize         = 1
	fieldTwoTypeBytesVarint = byte(fieldTwoTypeBytesTag)
)

/*
LabelValues has a map field, so it's equivalent to:

    message LabelValues {
      message ValuesEntry {
        string key = 1;
        StringValues value = 2;
      }
      repeated ValuesEntry Values = 1;
    }
    message StringValues {
      repeated string Values = 1;
    }

The field numbers for LabelValues.Values and StringValues.Values are checked in
TestLabelValues and TestStringValues.
*/

// LabelValuesSizeFromMap implements Size from [GogoprotoCustomType] for a
// map[string][]string acting as a wrappers.LabelValues.
func LabelValuesSizeFromMap[S ~[]string](m map[string]S) int {
	// each entry has one byte of tag for the implicit map entry message, plus
	// the size of the data as varint and the data itself
	size := len(m) * fieldOneTagSize

	for key, values := range m {
		// each string in the StringValues has one byte of tag, plus the size of
		// the data and the data
		sizeOfStringValues := len(values) * fieldOneTagSize
		for _, val := range values {
			sizeOfStringValues += protowire.SizeBytes(len(val))
		}
		// one byte of tag for the string key and one for the StringValues
		// value, plus the sizes of the data and then the data
		sizeOfMapEntry := fieldOneTagSize + protowire.SizeBytes(len(key)) + fieldTwoTagSize + protowire.SizeBytes(sizeOfStringValues)

		size += protowire.SizeBytes(sizeOfMapEntry)
	}

	return size
}

// LabelValuesMarshalFromMap implements Marshal from [GogoprotoCustomType] for a
// map[string][]string acting as a wrappers.LabelValues.
func LabelValuesMarshalFromMap[S ~[]string](m map[string]S) ([]byte, error) {
	size := LabelValuesSizeFromMap(m)
	data := make([]byte, size)
	n, err := LabelValuesMarshalToFromMap(m, data)
	if err != nil {
		return nil, err
	}
	return data[:n], nil
}

// LabelValuesMarshalToFromMap implements MarshalTo from [GogoprotoCustomType]
// for a map[string][]string acting as a wrappers.LabelValues.
func LabelValuesMarshalToFromMap[S ~[]string](m map[string]S, data []byte) (int, error) {
	data = data[:0:len(data)]

	for key, values := range m {
		// we have to calculate the sizes beforehand, as everything is prefixed
		// by its varint length, so the size calculations happen in the opposite
		// order as the actual encoding
		sizeOfStringValues := len(values) * fieldOneTagSize
		for _, val := range values {
			sizeOfStringValues += protowire.SizeBytes(len(val))
		}

		sizeOfMapEntry := fieldOneTagSize + protowire.SizeBytes(len(key)) + fieldTwoTagSize + protowire.SizeBytes(sizeOfStringValues)

		if cap(data)-len(data) < fieldOneTagSize+protowire.SizeBytes(sizeOfMapEntry) {
			// unfortunately gogoproto messages don't use MarshalToSizedBuffer
			// for custom types so we can't make assumptions about the length of
			// data being exact to the size that was calculated, the best we can
			// do without requiring yet another size calculation every time is
			// to check after every entry as we're about to append to the slice
			return 0, trace.BadParameter("LabelValues MarshalTo got a short buffer (this is a bug)")
		}

		// the size of the map entry
		data = append(data, fieldOneTypeBytesVarint)
		data = protowire.AppendVarint(data, uint64(sizeOfMapEntry))

		// the data of the map entry
		{
			// the map key
			data = append(data, fieldOneTypeBytesVarint)
			data = protowire.AppendString(data, key)

			// the size of the StringValues map value
			data = append(data, fieldTwoTypeBytesVarint)
			data = protowire.AppendVarint(data, uint64(sizeOfStringValues))

			// the data of the StringValues map value
			for _, val := range values {
				data = append(data, fieldOneTypeBytesVarint)
				data = protowire.AppendString(data, val)
			}
		}
	}

	return len(data), nil
}
