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
)

// NewPINCachingPrompt returns a new pin caching HardwareKeyPrompt.
// If [innerPrompt] already is a PIN caching prompt, it will be
// returned with an updated [cacheTTL].
func NewPINCachingPrompt(innerPrompt Prompt, cacheTTL time.Duration) *PINCachingPrompt {
	if p, ok := innerPrompt.(*PINCachingPrompt); ok {
		p.mu.Lock()
		defer p.mu.Unlock()

		p.cacheTTL = cacheTTL
		return p
	}

	return &PINCachingPrompt{
		Prompt:   innerPrompt,
		cacheTTL: cacheTTL,
	}
}

// PINCachingPrompt is a [Prompt] wrapped with PIN caching.
type PINCachingPrompt struct {
	// Prompt is the inner prompt used to prompt the user for touch
	// or PIN when it is not cached or is expired.
	Prompt

	// mu currently protects all fields.
	mu sync.Mutex

	// cacheTTL is the configured duration that a cached PIN will be valid.
	cacheTTL time.Duration
	// pin is the cached PIN.
	pin string
	// pinExpiry is the expiration time of the currently cached PIN.
	pinExpiry time.Time
}

// AskPIN returned the cached PIN if it is not expired. Otherwise, it uses
// the inner prompt to prompt the user for PIN, caching and returning it.
func (p *PINCachingPrompt) AskPIN(ctx context.Context, requirement PINPromptRequirement, keyInfo ContextualKeyInfo) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pin != "" && time.Now().Before(p.pinExpiry) {
		return p.pin, nil
	}

	pin, err := p.Prompt.AskPIN(ctx, requirement, keyInfo)
	if err != nil {
		return "", trace.Wrap(err)
	}

	p.pin = pin
	p.pinExpiry = time.Now().Add(p.cacheTTL)

	return pin, nil
}

// ChangePIN uses the inner prompt to prompt the user to change their PIN, then it caches the PIN
func (p *PINCachingPrompt) ChangePIN(ctx context.Context, keyInfo ContextualKeyInfo) (*PINAndPUK, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	PINAndPUK, err := p.Prompt.ChangePIN(ctx, keyInfo)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.pin = PINAndPUK.PIN
	p.pinExpiry = time.Now().Add(p.cacheTTL)

	return PINAndPUK, nil
}
