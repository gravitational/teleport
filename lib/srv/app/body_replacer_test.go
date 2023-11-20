/*
Copyright 2023 Gravitational, Inc.

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

package app

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_bytesReplacingReader(t *testing.T) {
	tests := []struct {
		name           string
		originalReader io.ReadCloser
		search         string
		replace        string
		want           string
	}{
		{
			name:           "empty",
			originalReader: newReaderCloser(),
			search:         "foo",
			replace:        "bar",
			want:           "",
		},
		{
			name:           "single string",
			originalReader: newReaderCloser("foo"),
			search:         "foo",
			replace:        "bar",
			want:           "bar",
		},
		{
			name:           "single string replace with a longer string",
			originalReader: newReaderCloser("foo"),
			search:         "foo",
			replace:        "bar2",
			want:           "bar2",
		},
		{
			name:           "single string replace with a empty string",
			originalReader: newReaderCloser("foo"),
			search:         "foo",
			replace:        "",
			want:           "",
		},
		{
			name:           "multi strings",
			originalReader: newReaderCloser("foo", "foo"),
			search:         "foo",
			replace:        "bar",
			want:           "barbar",
		},
		{
			name:           "multi strings partial match",
			originalReader: newReaderCloser("hello I am f", "oo"),
			search:         "foo",
			replace:        "bar",
			want:           "hello I am bar",
		},
		{
			name:           "multi strings partial match",
			originalReader: newReaderCloser("hello I am f", "o", "o"),
			search:         "foo",
			replace:        "bar",
			want:           "hello I am bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := newBytesReplacingReader(tt.originalReader, []byte(tt.search), []byte(tt.replace))
			require.NoError(t, err)
			got, err := io.ReadAll(b)
			require.NoError(t, err)
			require.Equal(t, tt.want, string(got))
		})
	}
}

func newReaderCloser(b ...string) io.ReadCloser {
	reader, writer := io.Pipe()
	go func() {
		defer writer.CloseWithError(io.EOF)
		for _, s := range b {
			_, err := writer.Write([]byte(s))
			if err != nil {
				panic(err)
			}
		}
	}()
	return reader
}
