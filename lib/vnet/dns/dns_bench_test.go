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

package dns

import (
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

func mustBuildQuery(b *testing.B, name string) []byte {
	b.Helper()
	msg := dnsmessage.Message{
		Header: dnsmessage.Header{ID: 1234, RecursionDesired: true},
		Questions: []dnsmessage.Question{{
			Name:  dnsmessage.MustNewName(name),
			Type:  dnsmessage.TypeA,
			Class: dnsmessage.ClassINET,
		}},
	}
	raw, err := msg.Pack()
	if err != nil {
		b.Fatalf("packing query: %v", err)
	}
	return raw
}

// go test ./lib/vnet/dns -run='^$' -bench=.
// goos: darwin
// goarch: arm64
// pkg: github.com/gravitational/teleport/lib/vnet/dns
// cpu: Apple M4 Pro
// BenchmarkDNSParseAndBuildA-14            9789584               112.7 ns/op             0 B/op          0 allocs/op
// PASS
// ok      github.com/gravitational/teleport/lib/vnet/dns  1.539s
func BenchmarkDNSParseAndBuildA(b *testing.B) {
	query := mustBuildQuery(b, "app.example.com.")
	respBuf := make([]byte, 0, maxUDPDNSMessageSize)
	addr := [4]byte{100, 64, 0, 2}

	b.ReportAllocs()
	for b.Loop() {
		var parser dnsmessage.Parser
		header, err := parser.Start(query)
		if err != nil {
			b.Fatalf("parsing header: %v", err)
		}
		question, err := parser.Question()
		if err != nil {
			b.Fatalf("parsing question: %v", err)
		}
		if _, err := buildAResponse(respBuf, &header, &question, addr); err != nil {
			b.Fatalf("building response: %v", err)
		}
	}
}
