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

package usagereporter

import (
	"encoding/json"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
)

func TestUserEventRequest_CheckAndSet(t *testing.T) {
	for _, tt := range []struct {
		name     string
		req      CreateUserEventRequest
		errCheck require.ErrorAssertionFunc
	}{
		{
			name: "event doesn't require extra data",
			req: CreateUserEventRequest{
				Event: bannerClickEvent,
			},
			errCheck: require.NoError,
		},
		{
			name: "event requires data and has data",
			req: CreateUserEventRequest{
				Event:     uiDiscoverStartedEvent,
				EventData: &json.RawMessage{},
			},
			errCheck: require.NoError,
		},
		{
			name: "event name is empty",
			req: CreateUserEventRequest{
				Event: "",
			},
			errCheck: require.Error,
		},
		{
			name: "event requires data but has no data",
			req: CreateUserEventRequest{
				Event:     uiDiscoverStartedEvent,
				EventData: nil,
			},
			errCheck: require.Error,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			subject := tt.req

			err := subject.CheckAndSetDefaults()
			tt.errCheck(t, err)
		})
	}
}

func TestConvertEventReqToUsageEvent(t *testing.T) {
	for _, tt := range []struct {
		name     string
		reqFn    func() CreateUserEventRequest
		errCheck require.ErrorAssertionFunc
		expected func() *usageeventsv1.UsageEventOneOf
	}{
		{
			name: "decodes discover started event",
			reqFn: func() CreateUserEventRequest {
				eventData := json.RawMessage(`{"id":"123", "stepStatus":"DISCOVER_STATUS_ERROR", "stepStatusError":"someerror"}`)
				return CreateUserEventRequest{
					Event:     uiDiscoverStartedEvent,
					EventData: &eventData,
				}
			},
			errCheck: require.NoError,
			expected: func() *usageeventsv1.UsageEventOneOf {
				return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverStartedEvent{
					UiDiscoverStartedEvent: &usageeventsv1.UIDiscoverStartedEvent{
						Metadata: &usageeventsv1.DiscoverMetadata{
							Id: "123",
						},
						Status: &usageeventsv1.DiscoverStepStatus{
							Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_ERROR,
							Error:  "someerror",
						},
					},
				}}
			},
		},
		{
			name: "error when invalid stepStatus",
			reqFn: func() CreateUserEventRequest {
				eventData := json.RawMessage(`{"id":"123", "stepStatus":"invalid", "stepStatusError":"someerror"}`)
				return CreateUserEventRequest{
					Event:     uiDiscoverStartedEvent,
					EventData: &eventData,
				}
			},
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, trace.IsBadParameter(err), "expected trace.BadParameter error, got: %v", err)
			},
		},
		{
			name: "decodes discover resource selected event",
			reqFn: func() CreateUserEventRequest {
				eventData := json.RawMessage(`
				{
					"id":"123",
					"resource":"DISCOVER_RESOURCE_SERVER",
					"stepStatus":"DISCOVER_STATUS_ERROR",
					"stepStatusError":"someerror"
				}
				`)
				return CreateUserEventRequest{
					Event:     uiDiscoverResourceSelectionEvent,
					EventData: &eventData,
				}
			},
			errCheck: require.NoError,
			expected: func() *usageeventsv1.UsageEventOneOf {
				return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiDiscoverResourceSelectionEvent{
					UiDiscoverResourceSelectionEvent: &usageeventsv1.UIDiscoverResourceSelectionEvent{
						Metadata: &usageeventsv1.DiscoverMetadata{
							Id: "123",
						},
						Resource: &usageeventsv1.DiscoverResourceMetadata{
							Resource: usageeventsv1.DiscoverResource_DISCOVER_RESOURCE_SERVER,
						},
						Status: &usageeventsv1.DiscoverStepStatus{
							Status: usageeventsv1.DiscoverStatus_DISCOVER_STATUS_ERROR,
							Error:  "someerror",
						},
					},
				}}
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()

			req := tt.reqFn()
			require.NoError(t, req.CheckAndSetDefaults())

			usageEvent, err := ConvertUserEventRequestToUsageEvent(req)
			tt.errCheck(t, err)
			if err != nil {
				return
			}

			require.Equal(t, tt.expected(), usageEvent)
		})
	}
}
