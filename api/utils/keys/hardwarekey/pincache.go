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

package hardwarekey

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// PINCache is a PIN cache that supports consumers with varying required TTLs.
type PINCache struct {
	Clock clockwork.Clock

	mu sync.Mutex
	// pin is the cached PIN.
	pin string
	// pinSetAt is the time when the cached PIN was set. Used to determine whether
	// the PIN should be considered expired for a specific caller's provided TTL.
	pinSetAt time.Time
	// pinExpiry is the expiration time of the cached PIN.
	pinExpiry time.Time
}

// NewPINCache returns a new PINCache.
func NewPINCache() *PINCache {
	return &PINCache{
		Clock: clockwork.NewRealClock(),
	}
}

// GetPIN retrieves the cached PIN. If the PIN was cached before by an amount of
// time equal to the provided TTL, the PIN will not be returned.
func (p *PINCache) GetPIN(ttl time.Duration) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pin == "" {
		return ""
	}

	// Check the pin cache expiration in case it hasn't been wiped by
	// the waitAndExpirePIN goroutine yet.
	if p.Clock.Now().After(p.pinExpiry) {
		return ""
	}

	// The PIN is cached, but does not satisfy the provided TTL of the request.
	// e.g. it has been alive for 8 minutes, but the provided TTL is 5 minutes.
	// For the purposes of this request, the pin should be considered expired.
	expiredForRequest := p.pinSetAt.Add(ttl)
	if !p.Clock.Now().Before(expiredForRequest) {
		return ""
	}

	return p.pin
}

// SetPIN sets the given PIN in the cache with the given TTL. If the PIN
// is already cached, the existing expiration is only updated if the given
// TTL would exceed that expiration.
func (p *PINCache) SetPIN(pin string, ttl time.Duration) {
	if ttl == 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	now := p.Clock.Now()
	expiry := now.Add(ttl)

	// PIN isn't already set or is being overwritten.
	if p.pin == "" || p.pin != pin {
		p.pin = pin
		p.pinSetAt = now
		p.pinExpiry = expiry

		// Start a goroutine to wipe the PIN once it's expired.
		go p.waitAndExpirePIN(expiry)
		return
	}

	p.pin = pin
	p.pinSetAt = p.Clock.Now()

	// Only set the expiration if it exceeds the current expiration.
	if expiry.After(p.pinExpiry) {
		p.pinExpiry = expiry
	}
}

// PromptOrGetPIN retrieves the cached PIN if set. Otherwise it prompts for the PIN and caches it.
func (p *PINCache) PromptOrGetPIN(ctx context.Context, prompt Prompt, requirement PINPromptRequirement, keyInfo ContextualKeyInfo, pinCacheTTL time.Duration) (string, error) {
	if pin := p.GetPIN(pinCacheTTL); pin != "" {
		return pin, nil
	}

	pin, err := prompt.AskPIN(ctx, requirement, keyInfo)
	if err != nil {
		return "", trace.Wrap(err)
	}

	p.SetPIN(pin, pinCacheTTL)
	return pin, nil
}

func (p *PINCache) waitAndExpirePIN(expiry time.Time) {
	for {
		p.Clock.Sleep(p.Clock.Until(expiry))

		p.mu.Lock()

		// If the expiration has been updated, keep waiting.
		if p.pinExpiry.After(expiry) {
			p.mu.Unlock()
			continue
		}

		p.pin = ""
		p.pinExpiry = time.Time{}
		p.pinSetAt = time.Time{}
		p.mu.Unlock()
	}
}
