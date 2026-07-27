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

package vnet

import (
	"io"
	"log/slog"
	"runtime"
	"testing"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// BenchmarkNetstackThroughput emulates an application's data path through the
// netstack and measures its performance at small and large chunk sizes.
//
// go test ./lib/vnet -run='^$' -bench=.
// goos: darwin
// goarch: arm64
// pkg: github.com/gravitational/teleport/lib/vnet
// cpu: Apple M4 Pro
// BenchmarkNetstackThroughput/smallChunks-14                 27142             46054 ns/op                64.08 allocs/KiB            1291 B/op         32 allocs/op
// BenchmarkNetstackThroughput/mediumChunks-14                18118             65857 ns/op                14.67 allocs/KiB            2234 B/op         58 allocs/op
// BenchmarkNetstackThroughput/largeChunks-14                   288           4143273 ns/op                 8.790 allocs/KiB         390954 B/op       9001 allocs/op
// PASS
// ok      github.com/gravitational/teleport/lib/vnet      4.967s
func BenchmarkNetstackThroughput(b *testing.B) {
	utils.InitLogger(utils.LoggingForCLI, slog.LevelError)
	b.Cleanup(func() { utils.InitLogger(utils.LoggingForCLI, slog.LevelDebug) })
	clock := clockwork.NewFakeClock()
	clientApp := newFakeClientApp(b.Context(), b, &fakeClientAppConfig{
		clusters: map[string]testClusterSpec{
			"root1.example.com": {
				apps:      []appSpec{{publicAddr: "echo.root1.example.com"}},
				cidrRange: "192.168.2.0/24",
			},
		},
		clock:                   clock,
		signatureAlgorithmSuite: types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1,
	})
	p := newTestPack(b, b.Context(), testPackConfig{
		fakeClientApp: clientApp,
		clock:         clock,
	})

	cases := []struct {
		name string
		size int
	}{
		{name: "smallChunks", size: 512},
		{name: "mediumChunks", size: 4 << 10},
		{name: "largeChunks", size: 1 << 20},
	}
	for _, bc := range cases {
		b.Run(bc.name, func(b *testing.B) {
			conn, err := p.dialHost(b.Context(), "echo.root1.example.com", 80)
			if err != nil {
				b.Fatalf("dialing echo app: %v", err)
			}
			b.Cleanup(func() { _ = conn.Close() })

			writeBuf := make([]byte, bc.size)
			readBuf := make([]byte, bc.size)
			startWrite := make(chan struct{})
			writeDone := make(chan error, 1)

			go func() {
				for range startWrite {
					_, err := conn.Write(writeBuf)
					writeDone <- err
				}
			}()
			b.Cleanup(func() { close(startWrite) })

			b.ReportAllocs()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			iters := 0
			for b.Loop() {
				startWrite <- struct{}{}
				if _, err := io.ReadFull(conn, readBuf); err != nil {
					b.Fatalf("reading: %v", err)
				}
				if err := <-writeDone; err != nil {
					b.Fatalf("writing: %v", err)
				}
				iters++
			}
			runtime.ReadMemStats(&after)

			kib := float64(bc.size) / 1024
			b.ReportMetric(float64(after.Mallocs-before.Mallocs)/float64(iters)/kib, "allocs/KiB")
		})
	}
}
