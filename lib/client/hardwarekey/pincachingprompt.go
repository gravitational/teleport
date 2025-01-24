/*
Copyright 2025 Gravitational, Inc.
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

package hardwarekey

import (
	"context"
	"time"

	"github.com/awnumar/memguard"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/keys"
)

// NewPinCachingPrompt returns a new pin caching HardwareKeyPrompt.
func NewPinCachingPrompt(innerPrompt keys.HardwareKeyPrompt, cacheTimeout time.Duration) *PinCachingPrompt {
	return &PinCachingPrompt{
		HardwareKeyPrompt: innerPrompt,
		PinCacheTimeout:   cacheTimeout,
	}
}

// PinCachingPrompt is a HardwareKeyPrompt wrapped with PIN caching.
type PinCachingPrompt struct {
	keys.HardwareKeyPrompt
	PinCacheTimeout time.Duration

	// We store the cached PIN in a secure enclave to make it maximally
	// difficult to extract from process memory or temporary disk storage.
	// See https://github.com/awnumar/memguard for more details on how this works.
	cachedPIN       *memguard.Enclave
	cachedPINExpiry time.Time
}

func (p *PinCachingPrompt) AskPIN(ctx context.Context, requirement keys.PINPromptRequirement) (string, error) {
	if pin, err := p.getCachedPIN(); err != nil {
		return "", trace.Wrap(err)
	} else if pin != "" {
		return pin, nil
	}

	pin, err := p.HardwareKeyPrompt.AskPIN(ctx, requirement)
	if err != nil {
		return "", trace.Wrap(err)
	}

	p.setCachedPIN(pin)
	return pin, nil
}

func (p *PinCachingPrompt) ChangePIN(ctx context.Context) (*keys.PINAndPUK, error) {
	PINAndPUK, err := p.HardwareKeyPrompt.ChangePIN(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.setCachedPIN(PINAndPUK.PIN)
	return PINAndPUK, nil
}

func (p *PinCachingPrompt) getCachedPIN() (string, error) {
	if p.cachedPIN != nil && time.Now().Before(p.cachedPINExpiry) {
		buf, err := p.cachedPIN.Open()
		if err != nil {
			return "", trace.Wrap(err)
		}
		defer buf.Destroy()
		return buf.String(), nil
	}

	return "", nil
}

func (p *PinCachingPrompt) setCachedPIN(pin string) {
	p.cachedPIN = memguard.NewEnclave([]byte(pin))
	p.cachedPINExpiry = time.Now().Add(p.PinCacheTimeout)
}
