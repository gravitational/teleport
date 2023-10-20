// Copyright 2023 Gravitational, Inc
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

package types_test

import (
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/types"
)

func TestValidateJamfSpecV1(t *testing.T) {
	validSpec := &types.JamfSpecV1{
		Enabled:     true,
		ApiEndpoint: "https://yourtenant.jamfcloud.com",
		Username:    "llama",
		Password:    "supersecret!!1!",
	}
	validEntry := &types.JamfInventoryEntry{
		FilterRsql:        "", // no filters
		SyncPeriodPartial: 0,  // default period
		SyncPeriodFull:    0,  // default period
		OnMissing:         "", // same as NOOP
	}

	modify := func(f func(spec *types.JamfSpecV1)) *types.JamfSpecV1 {
		spec := proto.Clone(validSpec).(*types.JamfSpecV1)
		f(spec)
		return spec
	}

	tests := []struct {
		name    string
		spec    *types.JamfSpecV1
		wantErr string
	}{
		{
			name: "minimal spec",
			spec: validSpec,
		},
		{
			name: "spec with inventory",
			spec: &types.JamfSpecV1{
				Enabled:     true,
				ApiEndpoint: "https://yourtenant.jamfcloud.com",
				Username:    "llama",
				Password:    "supersecret!!1!",
				Inventory: []*types.JamfInventoryEntry{
					{
						FilterRsql:        `general.remoteManagement.managed==true and general.platform=="Mac"`,
						SyncPeriodPartial: types.Duration(4 * time.Hour),
						SyncPeriodFull:    types.Duration(48 * time.Hour),
						OnMissing:         "DELETE",
					},
					{
						FilterRsql: `general.remoteManagement.managed==false`,
						OnMissing:  "NOOP",
					},
					validEntry,
				},
			},
		},
		{
			name: "all fields",
			spec: &types.JamfSpecV1{
				Enabled:     true,
				Name:        "jamf2",
				SyncDelay:   types.Duration(2 * time.Minute),
				ApiEndpoint: "https://yourtenant.jamfcloud.com",
				Username:    "llama",
				Password:    "supersecret!!1!",
				Inventory: []*types.JamfInventoryEntry{
					{
						FilterRsql:        `general.remoteManagement.managed==true and general.platform=="Mac"`,
						SyncPeriodPartial: types.Duration(4 * time.Hour),
						SyncPeriodFull:    types.Duration(48 * time.Hour),
						OnMissing:         "DELETE",
					},
				},
			},
		},
		{
			name:    "nil spec",
			spec:    nil,
			wantErr: "spec required",
		},
		{
			name: "api_endpoint invalid",
			spec: modify(func(spec *types.JamfSpecV1) {
				spec.ApiEndpoint = "https://%%"
			}),
			wantErr: "API endpoint",
		},
		{
			name: "api_endpoint empty hostname",
			spec: modify(func(spec *types.JamfSpecV1) {
				spec.ApiEndpoint = "not a valid URL"
			}),
			wantErr: "missing hostname",
		},
		{
			name: "username empty",
			spec: modify(func(spec *types.JamfSpecV1) {
				spec.Username = ""
			}),
			wantErr: "username",
		},
		{
			name: "password empty",
			spec: modify(func(spec *types.JamfSpecV1) {
				spec.Password = ""
			}),
			wantErr: "password",
		},
		{
			name: "inventory nil entry",
			spec: modify(func(spec *types.JamfSpecV1) {
				spec.Inventory = []*types.JamfInventoryEntry{
					nil,
				}
			}),
			wantErr: "is nil",
		},
		{
			name: "inventory sync_partial > sync_full",
			spec: modify(func(spec *types.JamfSpecV1) {
				spec.Inventory = []*types.JamfInventoryEntry{
					validEntry,
					{
						SyncPeriodPartial: types.Duration(12 * time.Hour),
						SyncPeriodFull:    types.Duration(8 * time.Hour),
					},
				}
			}),
			wantErr: "greater or equal to sync_period_full",
		},
		{
			name: "inventory on_missing invalid",
			spec: modify(func(spec *types.JamfSpecV1) {
				spec.Inventory = []*types.JamfInventoryEntry{
					validEntry,
					{
						OnMissing: "BANANA",
					},
				}
			}),
			wantErr: "on_missing",
		},
		{
			name: "inventory sync_partial disabled",
			spec: modify(func(spec *types.JamfSpecV1) {
				spec.Inventory = []*types.JamfInventoryEntry{
					validEntry,
					{
						SyncPeriodPartial: -1,
						SyncPeriodFull:    types.Duration(8 * time.Hour),
					},
				}
			}),
		},
		{
			name: "inventory sync_full disabled",
			spec: modify(func(spec *types.JamfSpecV1) {
				spec.Inventory = []*types.JamfInventoryEntry{
					validEntry,
					{
						SyncPeriodPartial: types.Duration(12 * time.Hour),
						SyncPeriodFull:    -1,
					},
				}
			}),
		},
		{
			name: "inventory all syncs disabled",
			spec: modify(func(spec *types.JamfSpecV1) {
				spec.Inventory = []*types.JamfInventoryEntry{
					validEntry,
					{
						SyncPeriodPartial: 0,
						SyncPeriodFull:    0,
					},
				}
			}),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := types.ValidateJamfSpecV1(test.spec)
			if test.wantErr == "" {
				assert.NoError(t, err, "ValidateJamfSpecV1 failed")
			} else {
				assert.ErrorContains(t, err, test.wantErr, "ValidateJamfSpecV1 error mismatch")
				assert.True(t, trace.IsBadParameter(err), "ValidateJamfSpecV1 returned non-BadParameter error: %T", err)
			}
		})
	}
}
