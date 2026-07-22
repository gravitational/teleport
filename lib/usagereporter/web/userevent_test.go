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
			errCheck: func(tt require.TestingT, err error, i ...any) {
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
		{
			name: "decodes ui access graph  crown jewel diff view event",
			reqFn: func() CreateUserEventRequest {
				eventData := json.RawMessage(`
				{
					"affected_resource_source":"TELEPORT",
					"affected_resource_type":"ssh"
				}
				`)
				return CreateUserEventRequest{
					Event:     uiAccessGraphCrownJewelDiffViewEvent,
					EventData: &eventData,
				}
			},
			errCheck: require.NoError,
			expected: func() *usageeventsv1.UsageEventOneOf {
				return &usageeventsv1.UsageEventOneOf{Event: &usageeventsv1.UsageEventOneOf_UiAccessGraphCrownJewelDiffView{
					UiAccessGraphCrownJewelDiffView: &usageeventsv1.UIAccessGraphCrownJewelDiffViewEvent{
						AffectedResourceSource: "TELEPORT",
						AffectedResourceType:   "ssh",
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
