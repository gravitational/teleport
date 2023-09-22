/*
Copyright 2020 Gravitational, Inc.

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

package workpool

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestFull runs a pool though a round of normal usage,
// and verifies expected state along the way:
// - A group of workers acquires leases, do some work, and release them.
// - A second group of workers receives leases as the first group finishes.
// - The expected amount of leases is in play after this churn.
// - Updating the target lease count has the expected effect.
func TestFull(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	p := NewPool(ctx)
	var wg sync.WaitGroup
	// signal channel to cause the first group of workers to
	// release their leases.
	g1done := make(chan struct{})
	// timeout channel indicating all of group one should
	// have acquired their leases.
	g1timeout := make(chan struct{})
	go func() {
		time.Sleep(time.Millisecond * 500)
		close(g1timeout)
	}()
	p.Set(200)
	// spawn first group of workers.
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			select {
			case l := <-p.Acquire():
				<-g1done
				l.Release()
			case <-g1timeout:
				t.Errorf("Timeout waiting for lease")
			}
			wg.Done()
		}()
	}
	<-g1timeout
	// no additional leases should exist
	select {
	case l := <-p.Acquire():
		t.Errorf("unexpected lease: %+v", l)
	default:
	}
	// spawn a second group of workers that won't be able to
	// acquire their leases until the first group is done.
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			select {
			case <-p.Acquire():
				// leak deliberately
			case <-time.After(time.Millisecond * 512):
				t.Errorf("Timeout waiting for lease")
			}
			wg.Done()
		}()
	}
	// signal first group is done
	close(g1done)
	// wait for second group to acquire leases.
	wg.Wait()
	// no additional leases should exist
	select {
	case l := <-p.Acquire():
		counts := l.loadCounts()
		t.Errorf("unexpected lease grant: %+v, counts=%+v", l, counts)
	case <-time.After(time.Millisecond * 128):
	}
	// make one additional lease available
	p.Set(201)
	select {
	case l := <-p.Acquire():
		l.Release()
	case <-time.After(time.Millisecond * 128):
		t.Errorf("timeout waiting for lease grant")
	}
}

// TestZeroed verifies that a zeroed pool stops granting
// leases as expected.
func TestZeroed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	p := NewPool(ctx)
	p.Set(1)
	var l Lease
	select {
	case l = <-p.Acquire():
		l.Release()
	case <-time.After(time.Millisecond * 128):
		t.Errorf("timeout waiting for lease grant")
	}
	p.Set(0)
	// modifications to counts are *ordered*, but asynchronous,
	// so we could actually receive a lease here if we don't sleep
	// briefly. if we opted for condvars instead of channels, this
	// issue could be avoided at the cost of more cumbersome
	// composition/cancellation.
	time.Sleep(time.Millisecond * 10)
	select {
	case l := <-p.Acquire():
		t.Errorf("unexpected lease grant: %+v", l)
	case <-time.After(time.Millisecond * 128):
	}
}
