// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

//go:build desktop_access_rdp

package rdpclient

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// progressiveFixtureCapture dumps each raw RFX Progressive (WireToSurface2)
// payload + sidecar metadata to disk so the wasm-side decoder's test
// harness can replay real captures and compare against the FreeRDP oracle.
//
// Off unless RDP_DUMP_PROGRESSIVE_FIXTURES is set. Writes to [progressiveFixtureDir] (a fixed dev path). One
// file per PDU + a tab-separated manifest. If the path can't be created or
// written to (e.g., read-only filesystem in production), capture self-
// disables for the remainder of the process with an stderr warning.
//
// Output layout:
//   <dir>/manifest.tsv  — tab-separated: seq, ts_ns, surface, codec_id,
//                         codec_context_id, pixel_format, origin_x,
//                         origin_y, byte_count, filename
//   <dir>/000001.bin    — raw bitmap_data payload, byte-exact
//   <dir>/000002.bin
//   …
//
// The harness consumes manifest.tsv to drive replay. One row per PDU so
// ordering is preserved (matters for codec-context state across frames).
type progressiveFixtureCapture struct {
	dir       string
	manifest  *os.File
	mu        sync.Mutex
	seq       atomic.Uint64
	startNs   int64
	disabled  atomic.Bool
}

// Fixed default capture path. Picked to land somewhere always writeable on
// macOS / Linux dev machines without colliding with anything Teleport
// expects. Change here if you want to capture elsewhere.
const progressiveFixtureDir = "/tmp/teleport-rfx-fixtures"

var progressiveCaptureOnce sync.Once
var progressiveCapture *progressiveFixtureCapture

func getProgressiveCapture() *progressiveFixtureCapture {
	progressiveCaptureOnce.Do(initProgressiveCapture)
	return progressiveCapture
}

func initProgressiveCapture() {
	// Off unless RDP_DUMP_PROGRESSIVE_FIXTURES is set: this does a synchronous
	// disk write per WireToSurface2 PDU (hundreds/sec under animation), which
	// both pollutes timing measurements and hammers the disk, so it must not run
	// by default. Set RDP_DUMP_PROGRESSIVE_FIXTURES=1 to capture oracle fixtures.
	if os.Getenv("RDP_DUMP_PROGRESSIVE_FIXTURES") == "" {
		return
	}
	dir := progressiveFixtureDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[progressive-capture] mkdir %q: %v — disabled\n", dir, err)
		return
	}
	manifestPath := filepath.Join(dir, "manifest.tsv")
	f, err := os.OpenFile(manifestPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[progressive-capture] open %q: %v — disabled\n", manifestPath, err)
		return
	}
	// Header only when the manifest is brand-new (we append, so detect via
	// current file size).
	if st, statErr := f.Stat(); statErr == nil && st.Size() == 0 {
		fmt.Fprintln(f, "seq\tts_ns\tsurface\tcodec_id\tctx_id\tpixel_format\torigin_x\torigin_y\tbytes\tfile")
	}
	progressiveCapture = &progressiveFixtureCapture{
		dir:      dir,
		manifest: f,
		startNs:  time.Now().UnixNano(),
	}
	fmt.Fprintf(os.Stderr, "[progressive-capture] dumping RFX Progressive fixtures to %s\n", dir)
}

// dumpWireToSurface2 writes a single payload + manifest row. Safe to call
// when capture is disabled (returns immediately).
func (c *progressiveFixtureCapture) dumpWireToSurface2(
	surfaceID, codecID, ctxID, pixelFormat, originX, originY uint32,
	payload []byte,
) {
	if c == nil || c.disabled.Load() {
		return
	}
	seq := c.seq.Add(1)
	name := fmt.Sprintf("%06d.bin", seq)
	path := filepath.Join(c.dir, name)
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[progressive-capture] write %q: %v — disabling further dumps\n", path, err)
		c.disabled.Store(true)
		return
	}
	// Manifest write — single-threaded by the mutex so rows aren't interleaved.
	// Format kept stable: any change here breaks downstream parsers.
	c.mu.Lock()
	fmt.Fprintf(
		c.manifest,
		"%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s\n",
		seq, time.Now().UnixNano(),
		surfaceID, codecID, ctxID, pixelFormat,
		originX, originY,
		len(payload), name,
	)
	c.mu.Unlock()
}
