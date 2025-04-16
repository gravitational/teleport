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

package hardwarekey_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

func TestPINCachingPrompt(t *testing.T) {
	ctx := context.Background()

	// Note: locally this test gets flaky around 10µs.
	cacheTTL := 10 * time.Millisecond
	cachingPrompt := hardwarekey.NewPINCachingPrompt(&randPINPrompt{}, cacheTTL)

	t.Run("AskPIN", func(t *testing.T) {
		// prompt and cache a new PIN for 100ms.
		cachedPIN, err := cachingPrompt.AskPIN(ctx, hardwarekey.PINRequired, hardwarekey.ContextualKeyInfo{})
		require.NoError(t, err)
		timer := time.NewTimer(cacheTTL)

		// Check that the PIN remains cached.
		for i := 0; i < 3; i++ {
			pin, err := cachingPrompt.AskPIN(ctx, hardwarekey.PINRequired, hardwarekey.ContextualKeyInfo{})
			require.NoError(t, err)
			require.Equal(t, cachedPIN, pin)
		}

		// Check that the PIN is not cached after 100ms.
		<-timer.C
		pin, err := cachingPrompt.AskPIN(ctx, hardwarekey.PINRequired, hardwarekey.ContextualKeyInfo{})
		require.NoError(t, err)
		require.NotEqual(t, cachedPIN, pin)
	})

	t.Run("ChangePIN", func(t *testing.T) {
		// ChangePIN should prompt and cache a new PIN for 100ms.
		pinAndPUK, err := cachingPrompt.ChangePIN(ctx, hardwarekey.ContextualKeyInfo{})
		require.NoError(t, err)
		cachedPIN := pinAndPUK.PIN
		timer := time.NewTimer(cacheTTL)

		// Check that the PIN remains cached.
		for i := 0; i < 3; i++ {
			pin, err := cachingPrompt.AskPIN(ctx, hardwarekey.PINRequired, hardwarekey.ContextualKeyInfo{})
			require.NoError(t, err)
			require.Equal(t, cachedPIN, pin)
		}

		// Check that the PIN is not cached after 100ms.
		<-timer.C
		pin, err := cachingPrompt.AskPIN(ctx, hardwarekey.PINRequired, hardwarekey.ContextualKeyInfo{})
		require.NoError(t, err)
		require.NotEqual(t, cachedPIN, pin)
	})

}

type randPINPrompt struct{}

func (p *randPINPrompt) AskPIN(ctx context.Context, requirement hardwarekey.PINPromptRequirement, keyInfo hardwarekey.ContextualKeyInfo) (string, error) {
	return p.randPIN(), nil
}

func (p *randPINPrompt) Touch(ctx context.Context, keyInfo hardwarekey.ContextualKeyInfo) error {
	return nil
}

func (p *randPINPrompt) ChangePIN(ctx context.Context, keyInfo hardwarekey.ContextualKeyInfo) (*hardwarekey.PINAndPUK, error) {
	return &hardwarekey.PINAndPUK{
		PIN: p.randPIN(),
	}, nil
}

func (p *randPINPrompt) ConfirmSlotOverwrite(ctx context.Context, message string, keyInfo hardwarekey.ContextualKeyInfo) (bool, error) {
	return false, nil
}

func (p *randPINPrompt) randPIN() string {
	return fmt.Sprintf("%08d", rand.IntN(100000000))
}
