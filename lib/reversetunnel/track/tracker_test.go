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

	"github.com/stretchr/testify/require"
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

func (s *simpleTestProxies) Discover(tracker *Tracker, lease *Lease) (ok bool) {
	proxy, ok := s.GetRandProxy()
	if !ok {
		panic("discovery called with no available proxies")
	}
	return s.ProxyLoop(tracker, lease, proxy)
}

func (s *simpleTestProxies) ProxyLoop(tracker *Tracker, lease *Lease, proxy testProxy) (ok bool) {
	defer lease.Release()
	timeout := time.After(proxy.life)

	if !lease.Claim(proxy.principals...) {
		return false
	}

	ticker := time.NewTicker(jitter(time.Millisecond * 100))
	defer ticker.Stop()

Loop:
	for {
		select {
		case <-ticker.C:
			if p, ok := s.GetRandProxy(); ok {
				tracker.TrackExpected(Proxy{Name: p.principals[0]})
			}
		case <-timeout:
			break Loop
		}
	}

	return ok
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

func TestBasic(t *testing.T) {
	const (
		timeout    = time.Second * 16
		proxyCount = 16
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	timeoutC := time.After(timeout)
	ticker := time.NewTicker(time.Millisecond * 100)
	t.Cleanup(ticker.Stop)
	tracker, err := New(ctx, Config{ClusterName: "test-cluster"})
	require.NoError(t, err)
	min, max := time.Duration(0), timeout
	var proxies simpleTestProxies
	proxies.AddRandProxies(proxyCount, min, max)
Discover:
	for {
		select {
		case lease := <-tracker.Acquire():
			go proxies.Discover(tracker, lease)
		case <-ticker.C:
			activeCount := tracker.activeCount()
			t.Logf("activeCount: %v", activeCount)
			if activeCount == proxyCount {
				break Discover
			}
		case <-timeoutC:
			t.Fatal("timeout")
		}
	}
}

func TestFullRotation(t *testing.T) {
	const (
		proxyCount = 8
		minConnA   = time.Second * 2
		maxConnA   = time.Second * 3
		minConnB   = time.Second * 24
		maxConnB   = time.Second * 25
		timeout    = time.Second * 30
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ticker := time.NewTicker(time.Millisecond * 100)
	t.Cleanup(ticker.Stop)

	var proxies simpleTestProxies
	proxies.AddRandProxies(proxyCount, minConnA, maxConnA)
	tracker, err := New(ctx, Config{ClusterName: "test-cluster"})
	require.NoError(t, err)

	timeoutC := time.After(timeout)
Loop0:
	for {
		select {
		case lease := <-tracker.Acquire():
			// get our "discovered" proxy in the foreground
			// to prevent race with the call to RemoveRandProxies
			// that comes after this loop.
			proxy, ok := proxies.GetRandProxy()
			if !ok {
				t.Fatal("failed to get test proxy")
			}
			go proxies.ProxyLoop(tracker, lease, proxy)
		case <-ticker.C:
			activeCount := tracker.activeCount()
			t.Logf("activeCount0: %v", activeCount)
			if activeCount == proxyCount {
				break Loop0
			}
		case <-timeoutC:
			t.Fatal("timeout")
		}
	}
	proxies.RemoveRandProxies(proxyCount)
Loop1:
	for {
		select {
		case <-ticker.C:
			activeCount := tracker.activeCount()
			t.Logf("activeCount1: %v", activeCount)
			if activeCount < 1 {
				break Loop1
			}
		case <-timeoutC:
			t.Fatal("timeout")
		}
	}
	proxies.AddRandProxies(proxyCount, minConnB, maxConnB)
Loop2:
	for {
		select {
		case lease := <-tracker.Acquire():
			go proxies.Discover(tracker, lease)
		case <-ticker.C:
			activeCount := tracker.activeCount()
			t.Logf("activeCount2: %v", activeCount)
			if activeCount >= proxyCount {
				break Loop2
			}
		case <-timeoutC:
			t.Fatal("timeout")
		}
	}
}

// TestUUIDHandling verifies that host UUIDs are correctly extracted
// from the expected teleport principal format, and that gossip messages
// consisting only of uuid don't create duplicate entries.
func TestUUIDHandling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	tracker, err := New(ctx, Config{ClusterName: "test-cluster"})
	require.NoError(t, err)

	lease := <-tracker.Acquire()
	require.True(t, lease.Claim("my-proxy.test-cluster"))
	require.Equal(t, "my-proxy", lease.claimName)

	tracker.TrackExpected(Proxy{Name: "my-proxy"})

	select {
	case <-tracker.Acquire():
		t.Error("received unexpected lease")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestIsClaimed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	tracker, err := New(ctx, Config{ClusterName: "test-cluster"})
	require.NoError(t, err)

	tracker.TrackExpected(Proxy{Name: "proxy1"}, Proxy{Name: "proxy2"})
	require.False(t, tracker.IsClaimed("proxy1.test-cluster"))

	lease := <-tracker.Acquire()

	ok := lease.Claim("proxy1.test-cluster")
	require.True(t, ok)

	require.True(t, tracker.IsClaimed("proxy1"))
	require.True(t, tracker.IsClaimed("proxy1.test-cluster"))
	require.False(t, tracker.IsClaimed("proxy2"))

	lease.Release()

	require.False(t, tracker.IsClaimed("proxy1"))
	require.False(t, tracker.IsClaimed("proxy2"))
}

func (t *Tracker) activeCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return len(t.claimed) + t.inflight
}

func TestProxyGroups(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	tracker, err := New(ctx, Config{ClusterName: "test-cluster"})
	require.NoError(t, err)

	tracker.SetConnectionCount(2)

	tracker.TrackExpected(
		Proxy{Name: "xa", Group: "x", Generation: 1},
		Proxy{Name: "xb", Group: "x", Generation: 1},
		Proxy{Name: "yc", Group: "y", Generation: 1},
		Proxy{Name: "yd", Group: "y", Generation: 1},
	)

	xa := <-tracker.Acquire()
	xb := <-tracker.Acquire()

	require.True(t, xa.Claim("xa"))
	require.True(t, xb.Claim("xb"))

	noAcquire := func() {
		select {
		case <-tracker.Acquire():
			t.Fatal("unexpected lease granted")
		case <-time.After(50 * time.Millisecond):
		}
	}

	noAcquire()

	tracker.TrackExpected(
		Proxy{Name: "xe", Group: "x", Generation: 2},
		Proxy{Name: "xf", Group: "x", Generation: 2},
	)

	yc := <-tracker.Acquire()
	yd := <-tracker.Acquire()

	require.True(t, yc.Claim("yc"))
	require.True(t, yd.Claim("yd"))

	noAcquire()

	tracker.SetConnectionCount(0)

	xe := <-tracker.Acquire()
	xf := <-tracker.Acquire()

	noAcquire()

	require.True(t, xe.Claim("xe"))
	require.True(t, xf.Claim("xf"))

	// releasing a proxy from a previous generation doesn't let a new connection
	// spawn
	xa.Release()
	noAcquire()

	// whereas releasing a proxy from a current generation does
	yc.Release()
	<-tracker.Acquire()
}
