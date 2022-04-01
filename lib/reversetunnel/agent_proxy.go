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

package reversetunnel

import (
	"math/rand"
	"sync"

	"github.com/gravitational/teleport/api/types"
	"github.com/jonboulle/clockwork"

	"github.com/google/go-cmp/cmp"
)

// NewProxiedServiceUpdater creates a new ProxiedServiceUpdater instance.
func NewProxiedServiceUpdater(clock clockwork.Clock) *ProxiedServiceUpdater {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	rand := rand.New(rand.NewSource(clock.Now().UnixNano()))
	return &ProxiedServiceUpdater{
		ids:     make([]string, 0),
		nonceID: rand.Uint64(),
		nonce:   1,
	}
}

// ProxiedServiceUpdater updates a proxied service with the proxies it is connected to.
type ProxiedServiceUpdater struct {
	ids     []string
	nonceID uint64
	nonce   uint64
	mu      sync.RWMutex
}

// Update updates a given proxied service with proxy ids, nonce id, and nonce.
func (u *ProxiedServiceUpdater) Update(service types.ProxiedService) {
	u.mu.RLock()
	defer u.mu.RUnlock()

	service.SetProxyIDs(u.ids)
	service.SetNonceID(u.nonceID)
	service.SetNonce(u.nonce)
}

// setProxies sets the proxy ids to set for a proxied service
func (u *ProxiedServiceUpdater) setProxiesIDs(ids []string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if cmp.Equal(u.ids, ids) {
		return
	}

	u.ids = make([]string, len(ids))
	copy(u.ids, ids)
	u.nonce++
}
