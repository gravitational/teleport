// Copyright 2025 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package piv

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

// pinCache is a PIN cache that supports consumers with varying required TTLs.
type pinCache struct {
	clock clockwork.Clock

	mu sync.Mutex
	// pin is the cached PIN.
	pin string
	// pinSetAt is the time when the cached PIN was set. Used to determine whether
	// the PIN should be considered expired for a specific caller's provided TTL.
	pinSetAt time.Time
	// pinExpiry is the expiration time of the cached PIN.
	pinExpiry time.Time
}

// newPINCache returns a new PINCache.
func newPINCache() *pinCache { //nolint:unused // used in yubikey.go with piv build constraint
	return &pinCache{
		clock: clockwork.NewRealClock(),
	}
}

// PromptOrGetPIN retrieves the cached PIN if set. Otherwise it prompts for the PIN and caches it.
func (p *pinCache) PromptOrGetPIN(ctx context.Context, prompt hardwarekey.Prompt, requirement hardwarekey.PINPromptRequirement, keyInfo hardwarekey.ContextualKeyInfo, pinCacheTTL time.Duration) (string, error) {
	// If the provided ttl is 0, it doesn't support caching, so we just prompt.
	if pinCacheTTL == 0 {
		return prompt.AskPIN(ctx, requirement, keyInfo)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if pin := p.getPIN(pinCacheTTL); pin != "" {
		return pin, nil
	}

	// Add a timeout to prevent an unanswered PIN prompt from holding the lock.
	const pinPromptTimeout = time.Minute
	ctx, cancel := context.WithTimeout(ctx, pinPromptTimeout)
	defer cancel()

	pin, err := prompt.AskPIN(ctx, requirement, keyInfo)
	if err != nil {
		return "", trace.Wrap(err)
	}

	p.setPIN(pin, pinCacheTTL)
	return pin, nil
}

// getPIN retrieves the cached PIN. If the PIN was cached before by an amount of
// time equal to the provided TTL, the PIN will not be returned.
// Must be called under [p.mu] lock.
func (p *pinCache) getPIN(ttl time.Duration) string {
	if p.pin == "" {
		return ""
	}

	// Check if the PIN cache is expired. If it is, wipe it.
	if p.clock.Now().After(p.pinExpiry) {
		p.pin = ""
		p.pinExpiry = time.Time{}
		p.pinSetAt = time.Time{}
		return ""
	}

	// The PIN is cached, but does not satisfy the provided TTL of the request.
	// e.g. it has been alive for 8 minutes, but the provided TTL is 5 minutes.
	// For the purposes of this request, the pin should be considered expired.
	if p.clock.Since(p.pinSetAt) >= ttl {
		return ""
	}

	return p.pin
}

// setPIN sets the given PIN in the cache with the given TTL. If the PIN
// is already cached, the existing expiration is only updated if the given
// TTL would exceed that expiration.
// Must be called under [p.mu] lock.
func (p *pinCache) setPIN(pin string, ttl time.Duration) {
	now := p.clock.Now()
	expiry := now.Add(ttl)

	// Only set the expiration if it exceeds the current expiration
	// or the cached PIN is being changed.
	if expiry.After(p.pinExpiry) || p.pin != pin {
		p.pinExpiry = expiry
	}

	p.pin = pin
	p.pinSetAt = now
}
