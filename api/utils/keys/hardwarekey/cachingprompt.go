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
	"time"

	"github.com/gravitational/trace"
)

// NewPinCachingPrompt returns a new pin caching HardwareKeyPrompt.
func NewPinCachingPrompt(innerPrompt Prompt, cacheTimeout time.Duration) *PinCachingPrompt {
	return &PinCachingPrompt{
		Prompt:          innerPrompt,
		PinCacheTimeout: cacheTimeout,
	}
}

// PinCachingPrompt is a [Prompt] wrapped with PIN caching.
type PinCachingPrompt struct {
	Prompt
	// PinCacheTimeout configures the duration that the PIN will be cached.
	PinCacheTimeout time.Duration

	cachedPIN       string
	cachedPINExpiry time.Time
}

func (p *PinCachingPrompt) AskPIN(ctx context.Context, requirement PINPromptRequirement, keyInfo ContextualKeyInfo) (string, error) {
	if p.cachedPIN != "" && time.Now().Before(p.cachedPINExpiry) {
		return p.cachedPIN, nil
	}

	pin, err := p.Prompt.AskPIN(ctx, requirement, keyInfo)
	if err != nil {
		return "", trace.Wrap(err)
	}

	p.cachedPIN = pin
	p.cachedPINExpiry = time.Now().Add(p.PinCacheTimeout)

	return pin, nil
}

func (p *PinCachingPrompt) ChangePIN(ctx context.Context, keyInfo ContextualKeyInfo) (*PINAndPUK, error) {
	PINAndPUK, err := p.Prompt.ChangePIN(ctx, keyInfo)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.cachedPIN = PINAndPUK.PIN
	p.cachedPINExpiry = time.Now().Add(p.PinCacheTimeout)

	return PINAndPUK, nil
}
