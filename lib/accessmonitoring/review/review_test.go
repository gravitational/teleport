/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package review

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pb "github.com/gravitational/teleport/api/client/proto"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/accessmonitoring"
)

const (
	handlerName = "test-handler"
)

func TestInitializeCache(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	mockClient := &mockClient{}
	cache := accessmonitoring.NewCache()
	handler, err := NewHandler(Config{
		HandlerName: handlerName,
		Client:      mockClient,
		Cache:       cache,
	})
	require.NoError(t, err)

	mockReq := &accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest{
		Subjects:            []string{types.KindAccessRequest},
		AutomaticReviewName: handler.HandlerName,
	}

	mockResp := []*accessmonitoringrulesv1.AccessMonitoringRule{
		newApprovedRule("test-rule", "condition"),
	}

	mockClient.On("ListAccessMonitoringRulesWithFilter", mock.Anything, mockReq).
		Return(mockResp, "", nil)

	require.NoError(t, handler.HandleAccessMonitoringRule(ctx, types.Event{
		Type: types.OpInit,
		Resource: types.NewWatchStatus([]types.WatchKind{
			{Kind: types.KindAccessMonitoringRule},
		}),
	}))
	require.Len(t, cache.Get(), 1)
	require.ElementsMatch(t, mockResp, cache.Get())
}

func TestHandleAccessMonitoringRule(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	mockClient := &mockClient{}
	cache := accessmonitoring.NewCache()
	handler, err := NewHandler(Config{
		HandlerName: handlerName,
		Client:      mockClient,
		Cache:       cache,
	})
	require.NoError(t, err)

	// Test add rule.
	rule := newApprovedRule("test-rule", `condition`)
	require.NoError(t, handler.HandleAccessMonitoringRule(ctx, types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToResourceWithLabels(rule),
	}))
	require.Len(t, cache.Get(), 1)
	require.True(t, proto.Equal(rule, cache.Get()[0]))

	// Test update rule.
	rule = newApprovedRule("test-rule", `condition-updated`)
	require.NoError(t, handler.HandleAccessMonitoringRule(ctx, types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToResourceWithLabels(rule),
	}))
	require.Len(t, cache.Get(), 1)
	require.True(t, proto.Equal(rule, cache.Get()[0]))

	// Test delete rule.
	require.NoError(t, handler.HandleAccessMonitoringRule(ctx, types.Event{
		Type:     types.OpDelete,
		Resource: types.Resource153ToResourceWithLabels(rule),
	}))
	require.Empty(t, cache.Get())

	// Test rule does not apply with invalid automatic approval name.
	rule = newApprovedRule("test-rule", `condition`)
	rule.Spec.AutomaticReview.Integration = "invalid"
	require.NoError(t, handler.HandleAccessMonitoringRule(ctx, types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToResourceWithLabels(rule),
	}))
	require.Empty(t, cache.Get())

	// Test rule does not apply with invalid state.
	rule = newApprovedRule("test-rule", `condition`)
	rule.Spec.DesiredState = "invalid"
	require.NoError(t, handler.HandleAccessMonitoringRule(ctx, types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToResourceWithLabels(rule),
	}))
	require.Empty(t, cache.Get())

	// Test rule does not apply with invalid subject.
	rule = newApprovedRule("test-rule", `condition`)
	rule.Spec.Subjects = []string{"invalid"}
	require.NoError(t, handler.HandleAccessMonitoringRule(ctx, types.Event{
		Type:     types.OpPut,
		Resource: types.Resource153ToResourceWithLabels(rule),
	}))
	require.Empty(t, cache.Get())
}

// TestConflictingRules verifies that when there are multiple matching rules
// with conflicting review decisions, the `DENIED` rule will take precedence.
func TestConflictingRules(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	testReqID := uuid.New().String()
	withSecretsFalse := false
	requesterUserName := "requester"
	approvedRule := newApprovedRule("approved-rule", "true")
	deniedRule := newDeniedRule("denied-rule", "true")

	// Configure both an approved and denied rule.
	cache := accessmonitoring.NewCache()
	cache.Put([]*accessmonitoringrulesv1.AccessMonitoringRule{
		approvedRule,
		deniedRule,
	})

	requester, err := types.NewUser(requesterUserName)
	require.NoError(t, err)

	client := &mockClient{}
	client.On("GetUser", mock.Anything, requesterUserName, withSecretsFalse).
		Return(requester, nil)

	review, err := newAccessReview(
		requesterUserName,
		deniedRule.GetMetadata().GetName(),
		deniedRule.GetSpec().GetAutomaticReview().GetDecision(),
		time.Time{},
	)
	require.NoError(t, err)

	client.On("SubmitAccessReview", mock.Anything, types.AccessReviewSubmission{
		RequestID: testReqID,
		Review:    review,
	}).Return(mock.Anything, nil)

	handler, err := NewHandler(Config{
		HandlerName: handlerName,
		Client:      client,
		Cache:       cache,
	})
	require.NoError(t, err)

	req, err := types.NewAccessRequest(testReqID, "requester", "role")
	require.NoError(t, err)

	event := types.Event{
		Type:     types.OpPut,
		Resource: req,
	}
	require.NoError(t, handler.HandleAccessRequest(ctx, event))
}

func TestScheduleRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	testReqID := uuid.New().String()
	testRuleName := "test-rule"
	withSecretsFalse := false
	requesterUserName := "requester"

	requester, err := types.NewUser(requesterUserName)
	require.NoError(t, err)

	testRule := newApprovedRule(
		testRuleName,
		`true`)

	testRule.Spec.Schedules = map[string]*accessmonitoringrulesv1.Schedule{
		"test-schedule": {
			Time: &accessmonitoringrulesv1.TimeSchedule{
				Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
					{
						Weekday: time.Monday.String(),
						Start:   "14:00",
						End:     "15:00",
					},
				},
			},
		},
	}

	cache := accessmonitoring.NewCache()
	cache.Put([]*accessmonitoringrulesv1.AccessMonitoringRule{testRule})

	tests := []struct {
		description  string
		setupMock    func(m *mockClient)
		creationTime time.Time
		assertErr    require.ErrorAssertionFunc
	}{
		{
			description: "test within schedule",
			setupMock: func(m *mockClient) {
				m.On("GetUser", mock.Anything, requesterUserName, withSecretsFalse).
					Return(requester, nil)

				review, err := newAccessReview(
					requesterUserName,
					testRuleName,
					types.RequestState_APPROVED.String(),
					time.Time{},
				)
				require.NoError(t, err)

				m.On("SubmitAccessReview", mock.Anything, types.AccessReviewSubmission{
					RequestID: testReqID,
					Review:    review,
				}).Return(mock.Anything, nil)
			},
			creationTime: time.Date(2025, time.August, 11, 14, 30, 0, 0, time.UTC),
			assertErr:    require.NoError,
		},
		{
			description: "test outside schedule",
			setupMock: func(m *mockClient) {
				m.On("GetUser", mock.Anything, requesterUserName, withSecretsFalse).
					Return(requester, nil)

				m.AssertNotCalled(t, "SubmitAccessReview")
			},
			creationTime: time.Date(2025, time.August, 11, 15, 30, 0, 0, time.UTC),
			assertErr:    require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			t.Parallel()

			client := &mockClient{}
			if test.setupMock != nil {
				test.setupMock(client)
			}

			handler, err := NewHandler(Config{
				HandlerName: handlerName,
				Client:      client,
				Cache:       cache,
			})
			require.NoError(t, err)

			req, err := types.NewAccessRequest(
				testReqID,
				requesterUserName,
				"role",
			)
			require.NoError(t, err)
			req.SetCreationTime(test.creationTime)

			test.assertErr(t, handler.HandleAccessRequest(ctx, types.Event{
				Type:     types.OpPut,
				Resource: req,
			}))
			client.AssertExpectations(t)
		})
	}
}

func TestResourceRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	testReqID := uuid.New().String()
	testRuleName := "test-rule"
	withSecretsFalse := false
	requesterUserName := "requester"

	requester, err := types.NewUser(requesterUserName)
	require.NoError(t, err)

	testRule := newApprovedRule(
		testRuleName,
		`access_request.spec.resource_labels_intersection["env"].contains("test")`)

	cache := accessmonitoring.NewCache()
	cache.Put([]*accessmonitoringrulesv1.AccessMonitoringRule{testRule})

	tests := []struct {
		description string
		setupMock   func(m *mockClient)
		assertErr   require.ErrorAssertionFunc
	}{
		{
			description: "test 0 requested resources",
			setupMock: func(m *mockClient) {
				m.On("GetUser", mock.Anything, requesterUserName, withSecretsFalse).
					Return(requester, nil)

				m.On("ListResources", mock.Anything, mock.Anything).
					Return(&types.ListResourcesResponse{
						Resources:  []types.ResourceWithLabels{},
						TotalCount: 0,
					}, nil)

				m.AssertNotCalled(t, "SubmitAccessReview")
			},
			assertErr: require.NoError,
		},
		{
			description: "test !matching resource labels",
			setupMock: func(m *mockClient) {
				m.On("GetUser", mock.Anything, requesterUserName, withSecretsFalse).
					Return(requester, nil)

				m.On("ListResources", mock.Anything, mock.Anything).
					Return(&types.ListResourcesResponse{
						Resources: []types.ResourceWithLabels{
							&types.ServerV2{
								Metadata: types.Metadata{
									Labels: map[string]string{"env": "prod"},
								},
							},
						},
						TotalCount: 1,
					}, nil)

				m.AssertNotCalled(t, "SubmitAccessReview")
			},
			assertErr: require.NoError,
		},
		{
			description: "test matching resource labels",
			setupMock: func(m *mockClient) {
				m.On("GetUser", mock.Anything, requesterUserName, withSecretsFalse).
					Return(requester, nil)

				m.On("ListResources", mock.Anything, mock.Anything).
					Return(&types.ListResourcesResponse{
						Resources: []types.ResourceWithLabels{
							&types.ServerV2{
								Metadata: types.Metadata{
									Labels: map[string]string{"env": "test"},
								},
							},
						},
						TotalCount: 1,
					}, nil)

				review, err := newAccessReview(
					requesterUserName,
					testRuleName,
					types.RequestState_APPROVED.String(),
					time.Time{},
				)
				require.NoError(t, err)

				m.On("SubmitAccessReview", mock.Anything, types.AccessReviewSubmission{
					RequestID: testReqID,
					Review:    review,
				}).Return(mock.Anything, nil)
			},
			assertErr: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			t.Parallel()

			client := &mockClient{}
			if test.setupMock != nil {
				test.setupMock(client)
			}

			handler, err := NewHandler(Config{
				HandlerName: handlerName,
				Client:      client,
				Cache:       cache,
			})
			require.NoError(t, err)

			req, err := types.NewAccessRequestWithResources(
				testReqID,
				requesterUserName,
				[]string{"role"},
				[]types.ResourceID{
					{
						ClusterName:     "test-cluster",
						Kind:            types.KindNode,
						Name:            "test-node",
						SubResourceName: types.SubKindTeleportNode,
					},
				},
			)
			require.NoError(t, err)

			test.assertErr(t, handler.HandleAccessRequest(ctx, types.Event{
				Type:     types.OpPut,
				Resource: req,
			}))
			client.AssertExpectations(t)
		})
	}
}

func TestHandleAccessRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	t.Cleanup(cancel)

	const (
		unapprovedUserName   = "unapproved-user"
		approvedUserName     = "approved-user"
		approvedUserTraitKey = "approved-trait-key"
		approvedUserTraitVal = "approved-trait-value"
		approvedRole         = "approved-role"
		testRuleName         = "test-rule"

		withSecretsFalse = false
	)

	testReqID := uuid.New().String()

	// Setup test rule
	cache := accessmonitoring.NewCache()

	rule := newApprovedRule(testRuleName,
		fmt.Sprintf(`
			contains_all(set("%s"), access_request.spec.roles) &&
			contains_any(user.traits["%s"], set("%s"))`,
			approvedRole, approvedUserTraitKey, approvedUserTraitVal))
	cache.Put([]*accessmonitoringrulesv1.AccessMonitoringRule{rule})

	// Setup approved user
	approvedUser, err := types.NewUser(approvedUserName)
	require.NoError(t, err)
	approvedUser.SetTraits(map[string][]string{
		approvedUserTraitKey: {approvedUserTraitVal},
	})

	// Setup unapproved user
	unapprovedUser, err := types.NewUser(unapprovedUserName)
	require.NoError(t, err)

	tests := []struct {
		description   string
		requester     string
		requestedRole string
		setupMock     func(m *mockClient)
		assertErr     require.ErrorAssertionFunc
	}{
		{
			description:   "test non-existent user",
			requester:     "non-existent-user",
			requestedRole: "non-existent-role",
			setupMock: func(m *mockClient) {
				m.On("GetUser", mock.Anything, "non-existent-user", withSecretsFalse).
					Return(nil, trace.NotFound("user not found"))
			},
			assertErr: func(t require.TestingT, err error, _ ...any) {
				require.ErrorContains(t, err, "user not found")
			},
		},
		{
			description:   "test approved user for unapproved role",
			requester:     approvedUserName,
			requestedRole: "unapproved-role",
			setupMock: func(m *mockClient) {
				m.On("GetUser", mock.Anything, approvedUserName, withSecretsFalse).
					Return(approvedUser, nil)

				m.AssertNotCalled(t, "SubmitAccessReview",
					"user is not automatically approved for this role")
			},
			assertErr: require.NoError,
		},
		{
			description:   "test unapproved user for approved role",
			requester:     unapprovedUserName,
			requestedRole: approvedRole,
			setupMock: func(m *mockClient) {
				m.On("GetUser", mock.Anything, unapprovedUserName, withSecretsFalse).
					Return(unapprovedUser, nil)

				m.AssertNotCalled(t, "SubmitAccessReview",
					"user is not automatically approved for this role")
			},
			assertErr: require.NoError,
		},
		{
			description:   "test approved user for approved role",
			requester:     approvedUserName,
			requestedRole: approvedRole,
			setupMock: func(m *mockClient) {
				m.On("GetUser", mock.Anything, approvedUserName, withSecretsFalse).
					Return(approvedUser, nil)

				review, err := newAccessReview(
					approvedUserName,
					testRuleName,
					types.RequestState_APPROVED.String(),
					time.Time{},
				)
				require.NoError(t, err)

				m.On("SubmitAccessReview", mock.Anything, types.AccessReviewSubmission{
					RequestID: testReqID,
					Review:    review,
				}).Return(mock.Anything, nil)
			},
			assertErr: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			client := &mockClient{}
			if test.setupMock != nil {
				test.setupMock(client)
			}

			handler, err := NewHandler(Config{
				HandlerName: handlerName,
				Client:      client,
				Cache:       cache,
			})
			require.NoError(t, err)

			req, err := types.NewAccessRequest(testReqID, test.requester, test.requestedRole)
			require.NoError(t, err)

			test.assertErr(t, handler.HandleAccessRequest(ctx, types.Event{
				Type:     types.OpPut,
				Resource: req,
			}))

			client.AssertExpectations(t)
		})
	}
}

func newApprovedRule(name, condition string) *accessmonitoringrulesv1.AccessMonitoringRule {
	return newReviewRule(name, condition, types.RequestState_APPROVED.String())
}

func newDeniedRule(name, condition string) *accessmonitoringrulesv1.AccessMonitoringRule {
	return newReviewRule(name, condition, types.RequestState_DENIED.String())
}

func newReviewRule(name, condition, decision string) *accessmonitoringrulesv1.AccessMonitoringRule {
	return &accessmonitoringrulesv1.AccessMonitoringRule{
		Kind: types.KindAccessMonitoringRule,
		Metadata: &headerv1.Metadata{
			Name: name,
		},
		Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
			Subjects:     []string{types.KindAccessRequest},
			Condition:    condition,
			DesiredState: types.AccessMonitoringRuleStateReviewed,
			AutomaticReview: &accessmonitoringrulesv1.AutomaticReview{
				Integration: handlerName,
				Decision:    decision,
			},
		},
	}
}

// mockClient is a mock implementation of the Teleport API client.
type mockClient struct {
	mock.Mock
}

func (m *mockClient) SubmitAccessReview(ctx context.Context, review types.AccessReviewSubmission) (types.AccessRequest, error) {
	// Expect zero value timestamp for testing.
	review.Review.Created = time.Time{}

	args := m.Called(ctx, review)
	return (types.AccessRequest)(nil), args.Error(1)
}

func (m *mockClient) ListAccessMonitoringRulesWithFilter(ctx context.Context, req *accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest) (
	[]*accessmonitoringrulesv1.AccessMonitoringRule,
	string,
	error,
) {
	// Expect zero value page size for testing.
	req.PageSize = 0

	args := m.Called(ctx, req)
	rules, ok := args.Get(0).([]*accessmonitoringrulesv1.AccessMonitoringRule)
	if ok {
		return rules, args.String(1), args.Error(2)
	}
	return nil, args.String(1), args.Error(2)
}

func (m *mockClient) GetUser(ctx context.Context, name string, withSecrets bool) (types.User, error) {
	args := m.Called(ctx, name, withSecrets)
	user, ok := args.Get(0).(types.User)
	if ok {
		return user, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockClient) ListResources(ctx context.Context, req pb.ListResourcesRequest) (*types.ListResourcesResponse, error) {
	args := m.Called(ctx, req)
	resp, ok := args.Get(0).(*types.ListResourcesResponse)
	if ok {
		return resp, args.Error(1)
	}
	return nil, args.Error(1)
}
