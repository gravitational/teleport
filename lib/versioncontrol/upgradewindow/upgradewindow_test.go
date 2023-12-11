/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package upgradewindow

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
)

// fakeDriver is used to inject custom behavior into a dummy Driver instance.
type fakeDriver struct {
	mu    sync.Mutex
	kind  string
	sync  func(context.Context, proto.ExportUpgradeWindowsResponse) error
	reset func(context.Context) error
}

func (d *fakeDriver) Kind() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.kind != "" {
		return d.kind
	}
	return "fake"
}

func (d *fakeDriver) SyncSchedule(ctx context.Context, rsp proto.ExportUpgradeWindowsResponse) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.sync != nil {
		return d.sync(ctx, rsp)
	}

	return nil
}

func (d *fakeDriver) ResetSchedule(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.reset != nil {
		return d.reset(ctx)
	}

	return nil
}

func (d *fakeDriver) withLock(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	fn()
}

func TestExporterBasics(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sc := make(chan context.Context)

	testEvents := make(chan testEvent, 1024)

	// set up fake export func that can be set to fail multiple times in sequence
	var exportCount int
	var exportFail bool
	var exportFlaky bool
	var exportLock sync.Mutex
	export := func(ctx context.Context, req proto.ExportUpgradeWindowsRequest) (rsp proto.ExportUpgradeWindowsResponse, err error) {
		if req.UpgraderKind != "fake" {
			panic("unexpected upgrader kind") // sanity check, shouldn't ever happen in practice
		}
		rsp.SystemdUnitSchedule = "fake-schedule"
		exportLock.Lock()
		exportCount++
		if exportFlaky && exportCount%2 == 0 {
			err = fmt.Errorf("fake-export-flaky")
		}
		if exportFail {
			err = fmt.Errorf("fake-export-fail")
		}
		exportLock.Unlock()
		return
	}

	driver := new(fakeDriver)

	driver.withLock(func() {
		driver.sync = func(ctx context.Context, rsp proto.ExportUpgradeWindowsResponse) error {
			if rsp.SystemdUnitSchedule != "fake-schedule" {
				panic("unexpected schedule value") // sanity check, shouldn't ever happen in practice
			}
			return nil
		}
	})

	exporter, err := NewExporter(ExporterConfig[context.Context]{
		Driver:                   driver,
		ExportFunc:               export,
		AuthConnectivitySentinel: sc,
		UnhealthyThreshold:       time.Millisecond * 200,
		ExportInterval:           time.Millisecond * 300,
		FirstExport:              time.Millisecond * 10,
		testEvents:               testEvents,
	})
	require.NoError(t, err)

	go exporter.Run()
	defer exporter.Close()

	// without connection sentinel, exporter is unable to make progress. eventually forces reset.
	awaitEvents(t, testEvents,
		expect(resetFromRun),
		deny(sentinelAcquired, exportAttempt),
	)

	s1, s1Cancel := context.WithCancel(ctx)

	// provide a connection sentinel
	sc <- s1

	// wait until sentinel is acquired
	awaitEvents(t, testEvents,
		expect(sentinelAcquired),
	)

	// everything should now appear healthy/normal for multiple export cycles
	awaitEvents(t, testEvents,
		expect(exportAttempt, exportSuccess, exportSuccess),
		deny(resetFromRun, resetFromExport, getExportErr, syncExportErr, sentinelLost),
	)

	// introduce intermittent sync failures
	driver.withLock(func() {
		var si int
		driver.sync = func(ctx context.Context, rsp proto.ExportUpgradeWindowsResponse) error {
			si++
			if si%2 == 0 {
				return fmt.Errorf("some-fake-error")
			}
			return nil
		}
	})

	// we should see intermittent failures, but no resets
	awaitEvents(t, testEvents,
		expect(syncExportErr, syncExportErr, exportSuccess, exportSuccess),
		deny(resetFromExport, resetFromRun, sentinelLost),
	)

	// remove intermittent sync failures
	driver.withLock(func() {
		driver.sync = nil
	})

	// drain remaining failures and ensure that we hit at least one success
	awaitEvents(t, testEvents,
		expect(exportSuccess),
		deny(resetFromExport, resetFromRun, sentinelLost),
		drain(true),
	)

	// introduce intermittent failure to the export fn
	exportLock.Lock()
	exportFlaky = true
	exportLock.Unlock()

	// we should see intermittent failures, but no resets
	awaitEvents(t, testEvents,
		expect(getExportErr, getExportErr, exportSuccess, exportSuccess),
		deny(resetFromExport, resetFromRun, sentinelLost),
	)

	// introduce persistent failure to the export fn
	exportLock.Lock()
	exportFlaky = false
	exportFail = true
	exportLock.Unlock()

	// drain remaining successes and wait for next failure
	awaitEvents(t, testEvents,
		expect(getExportErr),
		deny(resetFromRun, sentinelLost),
		drain(true),
	)

	// ensure that we now observe frequent resets and no successes
	awaitEvents(t, testEvents,
		expect(resetFromExport, resetFromExport),
		deny(resetFromRun, sentinelLost, exportSuccess),
	)

	// clear export fail state
	exportLock.Lock()
	exportFail = false
	exportLock.Unlock()

	// terminate our first connection sentinel
	s1Cancel()

	// wait until we lose the sentinel
	awaitEvents(t, testEvents,
		expect(sentinelLost),
	)

	// we should revert to periodic resets
	awaitEvents(t, testEvents,
		expect(resetFromRun),
		deny(sentinelAcquired, exportAttempt),
	)

	// provide another sentinel
	s2, s2Cancel := context.WithCancel(ctx)
	sc <- s2

	// healthy operation should resume
	awaitEvents(t, testEvents,
		expect(sentinelAcquired, exportSuccess),
		deny(resetFromExport, exportFailure),
	)

	s2Cancel()
}

type eventOpts struct {
	expect map[testEvent]int
	deny   map[testEvent]struct{}
	drain  bool
}

type eventOption func(*eventOpts)

func expect(events ...testEvent) eventOption {
	return func(opts *eventOpts) {
		for _, event := range events {
			opts.expect[event] = opts.expect[event] + 1
		}
	}
}

func deny(events ...testEvent) eventOption {
	return func(opts *eventOpts) {
		for _, event := range events {
			opts.deny[event] = struct{}{}
		}
	}
}

func drain(d bool) eventOption {
	return func(opts *eventOpts) {
		opts.drain = d
	}
}

func awaitEvents(t *testing.T, ch <-chan testEvent, opts ...eventOption) {
	options := eventOpts{
		expect: make(map[testEvent]int),
		deny:   make(map[testEvent]struct{}),
	}
	for _, opt := range opts {
		opt(&options)
	}

	if options.drain {
		drainEvents(t, ch, options)
	}

	timeout := time.After(time.Second * 5)
	for {
		if len(options.expect) == 0 {
			return
		}

		select {
		case event := <-ch:
			if _, ok := options.deny[event]; ok {
				require.Failf(t, "unexpected event", "event=%v", event)
			}

			options.expect[event] = options.expect[event] - 1
			if options.expect[event] < 1 {
				delete(options.expect, event)
			}
		case <-timeout:
			require.Failf(t, "timeout waiting for events", "expect=%+v", options.expect)
		}
	}
}

func drainEvents(t *testing.T, ch <-chan testEvent, options eventOpts) {
	timeout := time.After(time.Second * 5)
	for {
		select {
		case event := <-ch:
			if _, ok := options.deny[event]; ok {
				require.Failf(t, "unexpected event", "event=%v", event)
			}
		case <-timeout:
			require.Fail(t, "timeout attempting to drain events channel")
		default:
			return
		}
	}
}
