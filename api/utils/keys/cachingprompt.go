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

package keys

import (
	"context"
	"time"

	"github.com/gravitational/trace"
)

// PinCachingPrompt is a HardwareKeyPrompt wrapped with PIN caching.
type PinCachingPrompt struct {
	HardwareKeyPrompt

	pinCacheTimeout time.Duration
	// TODO: cache safely, not plainly in memory.
	cachedPIN       string
	cachedPINExpiry time.Time
}

func (p *PinCachingPrompt) AskPIN(ctx context.Context, requirement PINPromptRequirement) (string, error) {
	if p.cachedPIN != "" && time.Now().Before(p.cachedPINExpiry) {
		return p.cachedPIN, nil
	}

	pin, err := p.HardwareKeyPrompt.AskPIN(ctx, requirement)
	if err != nil {
		return "", trace.Wrap(err)
	}

	p.cachedPIN = pin
	p.cachedPINExpiry = time.Now().Add(p.pinCacheTimeout)

	return pin, nil
}

func (p *PinCachingPrompt) ChangePIN(ctx context.Context) (*PINAndPUK, error) {
	PINAndPUK, err := p.HardwareKeyPrompt.ChangePIN(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	p.cachedPIN = PINAndPUK.PIN
	p.cachedPINExpiry = time.Now().Add(p.pinCacheTimeout)

	return PINAndPUK, nil
}
