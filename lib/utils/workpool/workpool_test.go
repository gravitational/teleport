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
	"fmt"
	"sync"
	"testing"
	"time"

	"gopkg.in/check.v1"
)

func Example() {
	pool := NewPool(context.TODO())
	defer pool.Stop()
	// create two keys with different target counts
	pool.Set("spam", 2)
	pool.Set("eggs", 1)
	// track how many workers are spawned for each key
	counts := make(map[string]int)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			lease := <-pool.Acquire()
			defer lease.Release()
			mu.Lock()
			counts[lease.Key().(string)]++
			mu.Unlock()
			// in order to demonstrate the differing spawn rates we need
			// work to take some time, otherwise pool will end up granting
			// leases in a "round robin" fashion.
			time.Sleep(time.Millisecond * 10)
			wg.Done()
		}()
	}
	wg.Wait()
	// exact counts will vary, but leases with key `spam`
	// will end up being generated approximately twice as
	// often as leases with key `eggs`.
	fmt.Println(counts["spam"] > counts["eggs"]) // Output: true
}

func Test(t *testing.T) {
	check.TestingT(t)
}

type WorkSuite struct{}

var _ = check.Suite(&WorkSuite{})

// TestFull runs a pool though a round of normal usage,
// and verifies expected state along the way:
// - A group of workers acquire leases, do some work, and release them.
// - A second group of workers receieve leases as the first group finishes.
// - The expected amout of leases are in play after this churn.
// - Updating the target lease count has the expected effect.
func (s *WorkSuite) TestFull(c *check.C) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	p := NewPool(ctx)
	key := "some-key"
	var wg sync.WaitGroup
	// signal channel to cause the first group of workers to
	// release their leases.
	g1done := make(chan struct{})
	// timeout channel indicating all of group one should
	// have acquired thier leases.
	g1timeout := make(chan struct{})
	go func() {
		time.Sleep(time.Millisecond * 500)
		close(g1timeout)
	}()
	p.Set(key, 200)
	// spawn first group of workers.
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			select {
			case l := <-p.Acquire():
				<-g1done
				l.Release()
			case <-g1timeout:
				c.Errorf("Timeout waiting for lease")
			}
			wg.Done()
		}()
	}
	<-g1timeout
	// no additional leases should exist
	select {
	case l := <-p.Acquire():
		c.Errorf("unexpected lease: %+v", l)
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
				c.Errorf("Timeout waiting for lease")
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
		c.Errorf("unexpected lease grant: %+v, counts=%+v", l, counts)
	case <-time.After(time.Millisecond * 128):
	}
	// make one additional lease available
	p.Set(key, 201)
	select {
	case l := <-p.Acquire():
		c.Assert(l.Key().(string), check.Equals, key)
		l.Release()
	case <-time.After(time.Millisecond * 128):
		c.Errorf("timeout waiting for lease grant")
	}
}

// TestZeroed varifies that a zeroed pool stops granting
// leases as expected.
func (s *WorkSuite) TestZeroed(c *check.C) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	p := NewPool(ctx)
	key := "some-key"
	p.Set(key, 1)
	var l Lease
	select {
	case l = <-p.Acquire():
		c.Assert(l.Key().(string), check.Equals, key)
		l.Release()
	case <-time.After(time.Millisecond * 128):
		c.Errorf("timeout waiting for lease grant")
	}
	p.Set(key, 0)
	// modifications to counts are *ordered*, but asynchronous,
	// so we could actually receieve a lease here if we don't sleep
	// briefly. if we opted for condvars instead of channels, this
	// issue could be avoided at the cost of more cumbersome
	// composition/cancellation.
	time.Sleep(time.Millisecond * 10)
	select {
	case l := <-p.Acquire():
		c.Errorf("unexpected lease grant: %+v", l)
	case <-time.After(time.Millisecond * 128):
	}
}
