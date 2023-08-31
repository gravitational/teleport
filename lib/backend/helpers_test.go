/*
Copyright 2018 Gravitational, Inc.

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

package backend

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

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
				Backend:  mockBackend{},
				LockName: "lock",
				TTL:      30 * time.Second,
			},
			want: LockConfiguration{
				Backend:       mockBackend{},
				LockName:      "lock",
				TTL:           30 * time.Second,
				RetryInterval: 250 * time.Millisecond,
			},
		},
		{
			name: "set RetryAcquireLockTimeout",
			in: LockConfiguration{
				Backend:       mockBackend{},
				LockName:      "lock",
				TTL:           30 * time.Second,
				RetryInterval: 10 * time.Second,
			},
			want: LockConfiguration{
				Backend:       mockBackend{},
				LockName:      "lock",
				TTL:           30 * time.Second,
				RetryInterval: 10 * time.Second,
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
			name: "missing lock name",
			in: LockConfiguration{
				Backend:  mockBackend{},
				LockName: "",
			},
			wantErr: "missing LockName",
		},
		{
			name: "missing TTL",
			in: LockConfiguration{
				Backend:  mockBackend{},
				LockName: "lock",
				TTL:      0,
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
			Backend:  mockBackend{},
			LockName: lockName,
			TTL:      ttl,
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
					Backend:       mockBackend{},
					LockName:      lockName,
					TTL:           ttl,
					RetryInterval: 250 * time.Millisecond,
				},
				ReleaseCtxTimeout: 300 * time.Millisecond,
				// defaults to halft of TTL.
				RefreshLockInterval: 30 * time.Second,
			},
		},
		{
			name: "errors from LockConfiguration is passed",
			input: func() RunWhileLockedConfig {
				cfg := minimumValidConfig
				cfg.LockName = ""
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
