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

package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func Test_OktaAssignmentStatus_fromProto_no_panic(t *testing.T) {
	toJSON := func(v any) string {
		d, _ := json.Marshal(v)
		return string(d)
	}

	require.NotPanics(t, func() {
		oktaAssignmentResourceStatusFromProto(OktaAssignmentStatusV1{})
	})

	seed := uint64(time.Now().UnixNano())
	for range 1000 {
		status := OktaAssignmentStatusV1{}
		FillValue(&status,
			WithSkipFieldsRandomly(seed, 5),
		)
		require.NotPanics(t, func() {
			_ = oktaAssignmentResourceStatusFromProto(status)
		}, "seed=%d\nval=%s", seed, toJSON(status))
	}
}

func Test_OktaAssignmentStatus_proto_conversion_round_trip(t *testing.T) {
	tests := []struct {
		name     string
		statusFn func(t *testing.T) OktaAssignmentStatus
	}{
		{
			name: "empty",
			statusFn: func(_ *testing.T) OktaAssignmentStatus {
				return OktaAssignmentStatus{}
			},
		},
		{
			name: "filled",
			statusFn: func(t *testing.T) OktaAssignmentStatus {
				s := OktaAssignmentStatus{}
				require.NoError(t, FillValue(&s))
				require.NotEmpty(t, s.Phase)
				require.NotEmpty(t, s.Targets.Status)
				require.NotEmpty(t, s.Targets.Status[0].Phase)
				return s
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := tt.statusFn(t)
			protoStatus := oktaAssignmentResourceStatusToProto(status)
			convertedStatus := oktaAssignmentResourceStatusFromProto(protoStatus)
			require.Empty(t, cmp.Diff(status, convertedStatus))
		})
	}

}
