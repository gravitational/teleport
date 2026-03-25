//go:build bpf && !386

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

package bpf

import (
	"context"
	"errors"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var logger = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentBPF)

const (
	// CommMax is the maximum length of a command from linux/sched.h.
	CommMax = 16

	// PathMax is the maximum length of a path from linux/limits.h.
	PathMax = 255

	// eventArg is an exec event that holds the arguments to a function.
	eventArg = 0

	// eventRet holds the return value and other data about an event.
	eventRet = 1
)

// Counter allows a BPF program to increment a Prometheus counter.
// The counter value is stored in a one element BPF array (map).
// When it's incremented, the BPF program also rings the doorbell
// via a ring buffer.
type Counter struct {
	// doorbellBuf contains dummy bytes and is used for signaling the userspace
	doorbellBuf *ringbuf.Reader

	// arr is a one element array containing the value
	arr *ebpf.Map
	// lastCnt keeps the last read counter value
	lastCnt uint64

	// wg is used to wait for the loop goroutine to finish
	wg sync.WaitGroup

	// counter is the associated Prometheus counter to increment
	counter prometheus.Counter
}

// NewCounter starts tracking the lost messages and updating the Prometheus counter.
func NewCounter(counter, doorbell *ebpf.Map, promCounter prometheus.Counter) (*Counter, error) {
	doorbellBuf, err := ringbuf.NewReader(doorbell)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c := &Counter{
		doorbellBuf: doorbellBuf,
		arr:         counter,
		counter:     promCounter,
	}

	c.wg.Go(c.loop)

	return c, nil
}

// Close will stop tracking and release the resources.
func (c *Counter) Close() error {
	err := c.doorbellBuf.Close()
	// wait for lostLoop to finish
	c.wg.Wait()
	return err
}

func (c *Counter) loop() {
	for {
		_, err := c.doorbellBuf.Read()
		if err != nil {
			if errors.Is(err, ringbuf.ErrClosed) {
				return
			}
			logger.ErrorContext(context.Background(), "Error reading from ring buffer", "error", err)
			return
		}

		var key int32 = 0
		var count uint64
		if err := c.arr.Lookup(&key, &count); err != nil {
			logger.ErrorContext(context.Background(), "Error reading array value at index 0", "error", err)
			continue
		}
		if delta := count - c.lastCnt; delta > 0 {
			c.counter.Add(float64(delta))
		}
		c.lastCnt = count
	}
}
