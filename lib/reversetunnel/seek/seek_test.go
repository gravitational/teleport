/*
Copyright 2019 Gravitational, Inc.

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

package seek

import (
	"context"
	"fmt"
	"gopkg.in/check.v1"
	pr "math/rand"
	"sync"
	"testing"
	"time"
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

func (s *simpleTestProxies) Discover(handle GroupHandle) (ok bool) {
	proxy, ok := s.GetRandProxy()
	if !ok {
		panic("discovery called with no available proxies")
	}
	timeout := time.After(proxy.life)
	ok = handle.WithProxy(func() {
		ticker := time.NewTicker(jitter(time.Millisecond * 256))
	Loop:
		for {
			select {
			case <-ticker.C:
				if p, ok := s.GetRandProxy(); ok {
					handle.Gossip() <- p.principals[0]
				}
			case <-timeout:
				break Loop
			}
		}
	}, proxy.principals...)
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

// prMillis generates a pseudorandom duration
// of [0,max) milliseconds.
func prMillis(min int64, max int64) time.Duration {
	n := time.Duration(pr.Int63n(max-min) + min)
	return n * time.Millisecond
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

func (s *StateSuite) TestBasicHealthy(c *check.C) {
	s.runBasicProxyTest(c, time.Second*16, false)
}

func (s *StateSuite) TestBasicUnhealthy(c *check.C) {
	s.runBasicProxyTest(c, time.Second*16, true)
}

func (s *StateSuite) runBasicProxyTest(c *check.C, timeout time.Duration, allowUnhealthy bool) {
	const proxyCount = 16
	timeoutC := time.After(timeout)
	conf := newConfigOK(jitter(time.Millisecond * 512))
	pool, err := NewPool(context.TODO(), conf)
	c.Assert(err, check.IsNil)
	defer pool.Shutdown()
	handle := pool.Group(Key{Cluster: "test-cluster"})
	min, max := time.Duration(0), timeout
	if !allowUnhealthy {
		min = conf.BackoffThreshold
	}
	if max <= min {
		min = timeout
		max = timeout + time.Millisecond
	}
	var proxies simpleTestProxies
	proxies.AddRandProxies(proxyCount, min, max)
Discover:
	for {
		select {
		case <-pool.Seek():
			go proxies.Discover(handle)
		case status := <-handle.Status():
			c.Logf("Status: %+v", status)
			if status.Sum() == proxyCount {
				break Discover
			}
		case <-timeoutC:
			panic("timeout")
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
	var proxies simpleTestProxies
	proxies.AddRandProxies(proxyCount, minConnA, maxConnA)
	conf := newConfigOK(jitter(time.Millisecond * 128))
	pool, err := NewPool(context.TODO(), conf)
	c.Assert(err, check.IsNil)
	defer pool.Shutdown()
	handle := pool.Group(Key{Cluster: "test-cluster"})
	timeoutC := time.After(timeout)
Loop0:
	for {
		select {
		case key := <-pool.Seek():
			c.Assert(key, check.DeepEquals, Key{Cluster: "test-cluster"})
			go proxies.Discover(handle)
		case status := <-handle.Status():
			c.Logf("Status0: %+v", status)
			if status.Sum() == proxyCount {
				break Loop0
			}
		case <-timeoutC:
			panic("timeout")
		}
	}
	proxies.RemoveRandProxies(proxyCount)
Loop1:
	for {
		select {
		case status := <-handle.Status():
			c.Logf("Status1: %+v", status)
			if status.Claimed < 1 {
				break Loop1
			}
		case <-timeoutC:
			panic("timeout")
		}
	}
	proxies.AddRandProxies(proxyCount, minConnB, maxConnB)
Loop2:
	for {
		select {
		case <-pool.Seek():
			go proxies.Discover(handle)
		case status := <-handle.Status():
			c.Logf("Status2: %+v", status)
			if status.Claimed >= proxyCount {
				break Loop2
			}
		case <-timeoutC:
			panic("timeout")
		}
	}
}

// TestUUIDHandling verifies that host UUIDs are correctly extracted
// from the expected teleport principal format, and that gossip messages
// consisting only of uuid don't create duplicate seek entries.
func (s *StateSuite) TestUUIDHandling(c *check.C) {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*6)
	defer cancel()
	conf := newConfigOK(time.Millisecond * 512)
	pool, err := NewPool(ctx, conf)
	c.Assert(err, check.IsNil)
	key := Key{Cluster: "test-cluster"}
	handle := pool.Group(key)

	// claim a proxy using principal of the form <uuid>.<cluster>
	go handle.WithProxy(func() {
		c.Logf("Successfully claimed proxy")
		<-ctx.Done()
	}, "my-proxy.test-cluster")

	// Wait for proxy to be claimed
Wait:
	for {
		select {
		case status := <-handle.Status():
			c.Logf("Status: %+v", status)
			if !status.ShouldSeek() {
				break Wait
			}
		case <-ctx.Done():
			c.Errorf("pool never reached expected state")
		}
	}

	// Send a gossip message containing host UUID only
	handle.Gossip() <- "my-proxy"
	c.Logf("Sent uuid-only gossip message; watching status...")

	// Let pool go through a few ticks, monitoring status to ensure that
	// we don't incorrectly enter seek mode (entering seek mode here would
	// indicate that a duplicate entry was created for the uuid-only gossip
	// message).
	for i := 0; i < 3; i++ {
		select {
		case status := <-handle.Status():
			c.Logf("Status: %+v", status)
			if status.ShouldSeek() {
				c.Errorf("pool incorrectly entered seek mode")
			}
		case <-ctx.Done():
			c.Errorf("timeout")
		}
	}
}

func (s *StateSuite) BenchmarkBasicSeek(c *check.C) {
	const proxyCount = 32
	var proxies simpleTestProxies
	proxies.AddRandProxies(proxyCount, time.Second*16, time.Second*32)
	conf := newConfigOK(time.Millisecond * 512)
	pool, err := NewPool(context.TODO(), conf)
	c.Assert(err, check.IsNil)
	defer pool.Shutdown()
	for i := 0; i < c.N; i++ {
		key := Key{Cluster: fmt.Sprintf("cluster-%d", i)}
		handle := pool.Group(key)
	Discover:
		for {
			select {
			case <-pool.Seek():
				go proxies.Discover(handle)
			case status := <-handle.Status():
				c.Logf("Status: %+v", status)
				if status.Sum() == proxyCount {
					break Discover
				}
			}
		}
		pool.Stop(key)
	}
}

func newConfigOK(tickRate time.Duration) Config {
	conf := Config{
		TickRate:         tickRate,
		EntryExpiry:      tickRate * 180,
		BackoffInterval:  tickRate / 4,
		BackoffThreshold: tickRate * 30,
	}
	if err := conf.Check(); err != nil {
		panic(err)
	}
	return conf
}
