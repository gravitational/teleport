/*
Copyright 2021 Gravitational, Inc.

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

package utils

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockResolver struct {
	addresses []net.IPAddr
	err       error
}

func (r *mockResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	if r.err != nil {
		return nil, r.err
	}

	return r.addresses, nil
}

func TestIsLoopback(t *testing.T) {
	testCases := []struct {
		desc     string
		addr     string
		resolver nameResolver
		expect   bool
	}{
		{
			desc:     "localhost should return true",
			addr:     "localhost",
			resolver: net.DefaultResolver,
			expect:   true,
		}, {
			desc:     "localhost with port should return true",
			addr:     "localhost:1234",
			resolver: net.DefaultResolver,
			expect:   true,
		}, {
			desc: "multiple loopback addresses should return true",
			addr: "potato.banana.org",
			resolver: &mockResolver{
				addresses: []net.IPAddr{
					{IP: net.IPv6loopback},
					{IP: []byte{127, 0, 0, 1}},
				},
			},
			expect: true,
		}, {
			desc:     "degenerate hostname should return false",
			addr:     ":1234",
			resolver: net.DefaultResolver,
			expect:   false,
		}, {
			desc:     "degenerate port should return true",
			addr:     "localhost:",
			resolver: net.DefaultResolver,
			expect:   true,
		}, {
			desc:     "DNS failure should return false",
			addr:     "potato.banana.org",
			resolver: &mockResolver{err: errors.New("kaboom")},
			expect:   false,
		}, {
			desc: "non-loopback addr should return false",
			addr: "potato.banana.org",
			resolver: &mockResolver{
				addresses: []net.IPAddr{
					{IP: []byte{192, 168, 0, 1}}, // private, but non-loopback
				},
			},
			expect: false,
		}, {
			desc: "Any non-looback address should return false",
			addr: "potato.banana.org",
			resolver: &mockResolver{
				addresses: []net.IPAddr{
					{IP: net.IPv6loopback},
					{IP: []byte{192, 168, 0, 1}}, // private, but non-loopback
					{IP: []byte{127, 0, 0, 1}},
				},
			},
			expect: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			require.Equal(t, tc.expect, isLoopbackWithResolver(tc.addr, tc.resolver))
		})
	}
}
