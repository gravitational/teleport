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

package web

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	v1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
)

func TestDiscoverEventDataToUsageEvent(t *testing.T) {
	for _, tt := range []struct {
		name     string
		event    string
		req      DiscoverEventData
		errCheck require.ErrorAssertionFunc
		expected *v1.UsageEventOneOf
	}{
		{
			name:     uiDiscoverStartedEvent + "/success",
			event:    uiDiscoverStartedEvent,
			errCheck: require.NoError,
			req: DiscoverEventData{
				ID:         "someid",
				StepStatus: "DISCOVER_STATUS_SUCCESS",
			},
			expected: &v1.UsageEventOneOf{Event: &v1.UsageEventOneOf_UiDiscoverStartedEvent{
				UiDiscoverStartedEvent: &v1.UIDiscoverStartedEvent{
					Metadata: &v1.DiscoverMetadata{Id: "someid"},
					Status:   &v1.DiscoverStepStatus{Status: v1.DiscoverStatus_DISCOVER_STATUS_SUCCESS},
				},
			}},
		},
		{
			name:     uiDiscoverResourceSelectionEvent + "/success",
			event:    uiDiscoverResourceSelectionEvent,
			errCheck: require.NoError,
			req: DiscoverEventData{
				ID:              "someid",
				Resource:        "DISCOVER_RESOURCE_SERVER",
				StepStatus:      "DISCOVER_STATUS_ERROR",
				StepStatusError: "Failed to fetch available resources",
			},
			expected: &v1.UsageEventOneOf{Event: &v1.UsageEventOneOf_UiDiscoverResourceSelectionEvent{
				UiDiscoverResourceSelectionEvent: &v1.UIDiscoverResourceSelectionEvent{
					Metadata: &v1.DiscoverMetadata{Id: "someid"},
					Resource: &v1.DiscoverResourceMetadata{Resource: v1.DiscoverResource_DISCOVER_RESOURCE_SERVER},
					Status: &v1.DiscoverStepStatus{
						Status: v1.DiscoverStatus_DISCOVER_STATUS_ERROR,
						Error:  "Failed to fetch available resources",
					},
				},
			}},
		},
		{
			name:  uiDiscoverResourceSelectionEvent + "/invalid resource",
			event: uiDiscoverResourceSelectionEvent,
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got %v", err)
			},
			req: DiscoverEventData{
				ID:              "someid",
				Resource:        "SERVER",
				StepStatus:      "DISCOVER_STATUS_ERROR",
				StepStatusError: "Failed to fetch available resources",
			},
		},
		{
			name:  uiDiscoverResourceSelectionEvent + "/invalid status",
			event: uiDiscoverResourceSelectionEvent,
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got %v", err)
			},
			req: DiscoverEventData{
				ID:              "someid",
				Resource:        "DISCOVER_RESOURCE_SERVER",
				StepStatus:      "ERROR",
				StepStatusError: "Failed to fetch available resources",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			got, err := tt.req.ToUsageEvent(tt.event)
			tt.errCheck(t, err)
			if err != nil {
				return
			}

			require.Equal(t, tt.expected, got)
		})
	}
}
