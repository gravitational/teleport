/*
Copyright 2022 Gravitational, Inc.

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

package ingress

import (
	"net"
	"sync"
	"testing"
)

func TestConntrackRace(t *testing.T) {
	wg := &sync.WaitGroup{}
	tracker := &conntrack{
		conns: make(map[net.Conn]struct{}),
	}
	for i := 0; i < 100; i++ {
		conn := &wrappedConn{}
		wg.Add(2)
		go func() {
			tracker.add(conn)
			wg.Done()
		}()
		go func() {
			tracker.remove(conn)
			wg.Done()
		}()
	}
	wg.Wait()
}
