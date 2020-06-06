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

package track

import (
	"context"
	"fmt"
	pr "math/rand"
	"sync"
	"testing"
	"time"

	"gopkg.in/check.v1"
)

type simpleTestProxies struct {
	sync.Mutex
	proxies []testProxy
}

func (s *simpleTestProxies) AddRandProxies(n int, min time.Duration, max time.Duration) {
	s.Lock()
	defer s.Unlock()
	for i := 0; i < n; i++ {
		proxy := newTestProxy(prDuration(min, max))
		s.proxies = append(s.proxies, proxy)
	}
}

func (s *simpleTestProxies) RemoveRandProxies(n int) {
	s.Lock()
	defer s.Unlock()
	if len(s.proxies) <= n {
		s.proxies = nil
		return
	}
	rms := make([]bool, len(s.proxies))
	rmc := 0
	for rmc < n {
		i := pr.Int() % len(s.proxies)
		if !rms[i] {
			rms[i] = true
			rmc++
		}
	}
	filtered := make([]testProxy, 0, len(s.proxies)-n)
	for i, p := range s.proxies {
		if !rms[i] {
			filtered = append(filtered, p)
		}
	}
	s.proxies = filtered
}

func (s *simpleTestProxies) GetRandProxy() (p testProxy, ok bool) {
	s.Lock()
	defer s.Unlock()
	if len(s.proxies) < 1 {
		ok = false
		return
	}
	i := pr.Int() % len(s.proxies)
	return s.proxies[i], true
}

func (s *simpleTestProxies) Discover(tracker *Tracker, lease Lease) (ok bool) {
	proxy, ok := s.GetRandProxy()
	if !ok {
		panic("discovery called with no available proxies")
	}
	return s.ProxyLoop(tracker, lease, proxy)
}

func (s *simpleTestProxies) ProxyLoop(tracker *Tracker, lease Lease, proxy testProxy) (ok bool) {
	defer lease.Release()
	timeout := time.After(proxy.life)
	ok = tracker.WithProxy(func() {
		ticker := time.NewTicker(jitter(time.Millisecond * 100))
	Loop:
		for {
			select {
			case <-ticker.C:
				if p, ok := s.GetRandProxy(); ok {
					tracker.TrackExpected(lease, p.principals[0])
				}
			case <-timeout:
				break Loop
			}
		}
	}, lease, proxy.principals...)
	return
}

type testProxy struct {
	principals []string
	life       time.Duration
}

func newTestProxy(life time.Duration) testProxy {
	principals := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		p := fmt.Sprintf("proxy-%d", pr.Int())
		principals = append(principals, p)
	}
	return testProxy{principals, life}
}

func prDuration(min time.Duration, max time.Duration) time.Duration {
	mn, mx := int64(min), int64(max)
	rslt := pr.Int63n(mx-mn) + mn
	return time.Duration(rslt)
}

func jitter(t time.Duration) time.Duration {
	maxJitter := t / 5
	baseJitter := time.Duration(pr.Uint64())
	j := baseJitter % maxJitter
	return t + j
}

func Test(t *testing.T) {
	pr.Seed(time.Now().UnixNano())
	check.TestingT(t)
}

type StateSuite struct{}

var _ = check.Suite(&StateSuite{})

func (s *StateSuite) TestBasic(c *check.C) {
	s.runBasicProxyTest(c, time.Second*16)
}

func (s *StateSuite) runBasicProxyTest(c *check.C, timeout time.Duration) {
	const proxyCount = 16
	timeoutC := time.After(timeout)
	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()
	tracker := New(context.TODO(), Config{})
	defer tracker.StopAll()
	key := Key{Cluster: "test-cluster"}
	tracker.Start(key)
	min, max := time.Duration(0), timeout
	var proxies simpleTestProxies
	proxies.AddRandProxies(proxyCount, min, max)
Discover:
	for {
		select {
		case lease := <-tracker.Acquire():
			go proxies.Discover(tracker, lease)
		case <-ticker.C:
			counts := tracker.wp.Get(key)
			c.Logf("Counts: %+v", counts)
			if counts.Active == proxyCount {
				break Discover
			}
		case <-timeoutC:
			c.Fatal("timeout")
		}
	}
}

func (s *StateSuite) TestFullRotation(c *check.C) {
	const (
		proxyCount = 8
		minConnA   = time.Second * 2
		maxConnA   = time.Second * 3
		minConnB   = time.Second * 24
		maxConnB   = time.Second * 25
		timeout    = time.Second * 30
	)
	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()
	var proxies simpleTestProxies
	proxies.AddRandProxies(proxyCount, minConnA, maxConnA)
	tracker := New(context.TODO(), Config{})
	defer tracker.StopAll()
	key := Key{Cluster: "test-cluster"}
	tracker.Start(key)
	timeoutC := time.After(timeout)
Loop0:
	for {
		select {
		case lease := <-tracker.Acquire():
			c.Assert(lease.Key().(Key), check.DeepEquals, key)
			// get our "discovered" proxy in the foreground
			// to prevent race with the call to RemoveRandProxies
			// that comes after this loop.
			proxy, ok := proxies.GetRandProxy()
			if !ok {
				c.Fatal("failed to get test proxy")
			}
			go proxies.ProxyLoop(tracker, lease, proxy)
		case <-ticker.C:
			counts := tracker.wp.Get(key)
			c.Logf("Counts0: %+v", counts)
			if counts.Active == proxyCount {
				break Loop0
			}
		case <-timeoutC:
			c.Fatal("timeout")
		}
	}
	proxies.RemoveRandProxies(proxyCount)
Loop1:
	for {
		select {
		case <-ticker.C:
			counts := tracker.wp.Get(key)
			c.Logf("Counts1: %+v", counts)
			if counts.Active < 1 {
				break Loop1
			}
		case <-timeoutC:
			c.Fatal("timeout")
		}
	}
	proxies.AddRandProxies(proxyCount, minConnB, maxConnB)
Loop2:
	for {
		select {
		case lease := <-tracker.Acquire():
			go proxies.Discover(tracker, lease)
		case <-ticker.C:
			counts := tracker.wp.Get(key)
			c.Logf("Counts2: %+v", counts)
			if counts.Active >= proxyCount {
				break Loop2
			}
		case <-timeoutC:
			c.Fatal("timeout")
		}
	}
}

// TestUUIDHandling verifies that host UUIDs are correctly extracted
// from the expected teleport principal format, and that gossip messages
// consisting only of uuid don't create duplicate entries.
func (s *StateSuite) TestUUIDHandling(c *check.C) {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*6)
	defer cancel()
	ticker := time.NewTicker(time.Millisecond * 10)
	defer ticker.Stop()
	tracker := New(ctx, Config{})
	defer tracker.StopAll()
	key := Key{Cluster: "test-cluster"}
	tracker.Start(key)
	lease := <-tracker.Acquire()
	// claim a proxy using principal of the form <uuid>.<cluster>
	go tracker.WithProxy(func() {
		c.Logf("Successfully claimed proxy")
		<-ctx.Done()
	}, lease, "my-proxy.test-cluster")

	// Wait for proxy to be claimed
Wait:
	for {
		select {
		case <-ticker.C:
			counts := tracker.wp.Get(key)
			c.Logf("Counts: %+v", counts)
			if counts.Active == counts.Target {
				break Wait
			}
		case <-ctx.Done():
			c.Errorf("pool never reached expected state")
		}
	}

	// Send a gossip message containing host UUID only
	tracker.TrackExpected(lease, "my-proxy")
	c.Logf("Sent uuid-only gossip message; watching status...")

	// Let pool go through a few ticks, monitoring status to ensure that
	// we don't incorrectly enter seek mode (entering seek mode here would
	// indicate that a duplicate entry was created for the uuid-only gossip
	// message).
	for i := 0; i < 3; i++ {
		select {
		case <-ticker.C:
			counts := tracker.wp.Get(key)
			c.Logf("Counts: %+v", counts)
			if counts.Active != counts.Target {
				c.Errorf("incorrectly entered seek mode")
			}
		case <-ctx.Done():
			c.Errorf("timeout")
		}
	}
}
