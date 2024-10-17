/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package backend

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLockKey(t *testing.T) {
	t.Run("empty parts", func(t *testing.T) {
		key := lockKey()
		assert.Equal(t, ".locks", key.String())
		assert.Equal(t, []string{".locks"}, key.Components())
	})

	t.Run("with parts", func(t *testing.T) {
		key := lockKey("test", "llama")
		assert.Equal(t, ".locks/test/llama", key.String())
		assert.Equal(t, []string{".locks", "test", "llama"}, key.Components())
	})
}

func TestLockConfiguration_CheckAndSetDefaults(t *testing.T) {
	type mockBackend struct {
		Backend
	}
	tests := []struct {
		name     string
		in, want LockConfiguration
		wantErr  string
	}{
		{
			name: "minimum valid",
			in: LockConfiguration{
				Backend:            mockBackend{},
				LockNameComponents: []string{"lock"},
				TTL:                30 * time.Second,
			},
			want: LockConfiguration{
				Backend:            mockBackend{},
				LockNameComponents: []string{"lock"},
				TTL:                30 * time.Second,
				RetryInterval:      250 * time.Millisecond,
			},
		},
		{
			name: "set RetryAcquireLockTimeout",
			in: LockConfiguration{
				Backend:            mockBackend{},
				LockNameComponents: []string{"lock"},
				TTL:                30 * time.Second,
				RetryInterval:      10 * time.Second,
			},
			want: LockConfiguration{
				Backend:            mockBackend{},
				LockNameComponents: []string{"lock"},
				TTL:                30 * time.Second,
				RetryInterval:      10 * time.Second,
			},
		},
		{
			name: "missing backend",
			in: LockConfiguration{
				Backend: nil,
			},
			wantErr: "missing Backend",
		},
		{
			name: "missing lock components",
			in: LockConfiguration{
				Backend:            mockBackend{},
				LockNameComponents: nil,
			},
			wantErr: "missing LockName",
		},
		{
			name: "missing TTL",
			in: LockConfiguration{
				Backend:            mockBackend{},
				LockNameComponents: []string{"lock"},
				TTL:                0,
			},
			wantErr: "missing TTL",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.in
			err := cfg.CheckAndSetDefaults()
			if tt.wantErr == "" {
				require.NoError(t, err, "CheckAndSetDefaults return unexpected err")
				require.Empty(t, cmp.Diff(tt.want, cfg))
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestRunWhileLockedConfigCheckAndSetDefaults(t *testing.T) {
	type mockBackend struct {
		Backend
	}
	lockName := "lock"
	ttl := 1 * time.Minute
	minimumValidConfig := RunWhileLockedConfig{
		LockConfiguration: LockConfiguration{
			Backend:            mockBackend{},
			LockNameComponents: []string{lockName},
			TTL:                ttl,
		},
	}
	tests := []struct {
		name    string
		input   func() RunWhileLockedConfig
		want    RunWhileLockedConfig
		wantErr string
	}{
		{
			name: "minimum valid config",
			input: func() RunWhileLockedConfig {
				return minimumValidConfig
			},
			want: RunWhileLockedConfig{
				LockConfiguration: LockConfiguration{
					Backend:            mockBackend{},
					LockNameComponents: []string{lockName},
					TTL:                ttl,
					RetryInterval:      250 * time.Millisecond,
				},
				ReleaseCtxTimeout: time.Second,
				// defaults to halft of TTL.
				RefreshLockInterval: 30 * time.Second,
			},
		},
		{
			name: "errors from LockConfiguration is passed",
			input: func() RunWhileLockedConfig {
				cfg := minimumValidConfig
				cfg.LockNameComponents = nil
				return cfg
			},
			wantErr: "missing LockName",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.input()
			err := cfg.CheckAndSetDefaults()
			if tt.wantErr == "" {
				require.NoError(t, err, "CheckAndSetDefaults return unexpected err")
				require.Empty(t, cmp.Diff(tt.want, cfg))
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}
