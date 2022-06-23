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
	"bytes"
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func FuzzParsePacket(f *testing.F) {
	testcases := [][]byte{
		{},
		[]byte("00000"),
	}

	for _, tc := range testcases {
		f.Add(tc)
	}

	f.Fuzz(func(t *testing.T, packet []byte) {
		r := bytes.NewReader(packet)
		require.NotPanics(t, func() {
			_, _ = ParsePacket(r)
		})
	})
}

func FuzzFetchMySQLVersion(f *testing.F) {
	testcases := [][]byte{
		{},
		[]byte("00000"),
	}

	for _, tc := range testcases {
		f.Add(tc)
	}

	f.Fuzz(func(t *testing.T, packet []byte) {
		r := bytes.NewReader(packet)
		require.NotPanics(t, func() {
			_, _ = FetchMySQLVersionInternal(context.TODO(), func(ctx context.Context, network, address string) (net.Conn, error) {
				return &buffTestReader{reader: r}, nil
			}, "")
		})
	})
}

// buffTestReader is a fake reader used for test where read only
// net.Conn is needed.
type buffTestReader struct {
	reader *bytes.Reader
	net.Conn
}

func (r *buffTestReader) Read(b []byte) (int, error) {
	return r.reader.Read(b)
}

func (r *buffTestReader) Close() error {
	return nil
}
