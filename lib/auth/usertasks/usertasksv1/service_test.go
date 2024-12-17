/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package usertasksv1

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/usertasks"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

func TestServiceAccess(t *testing.T) {
	t.Parallel()
	testReporter := &mockUsageReporter{}

	testCases := []struct {
		name          string
		allowedVerbs  []string
		allowedStates []authz.AdminActionAuthState
	}{
		{
			name:         "CreateUserTask",
			allowedVerbs: []string{types.VerbCreate},
		},
		{
			name:         "UpdateUserTask",
			allowedVerbs: []string{types.VerbUpdate},
		},
		{
			name:         "DeleteUserTask",
			allowedVerbs: []string{types.VerbDelete},
		},
		{
			name:         "UpsertUserTask",
			allowedVerbs: []string{types.VerbCreate, types.VerbUpdate},
		},
		{
			name:         "ListUserTasks",
			allowedVerbs: []string{types.VerbRead, types.VerbList},
		},
		{
			name:         "ListUserTasksByIntegration",
			allowedVerbs: []string{types.VerbRead, types.VerbList},
		},
		{
			name:         "GetUserTask",
			allowedVerbs: []string{types.VerbRead},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			for _, verbs := range utils.Combinations(tt.allowedVerbs) {
				t.Run(fmt.Sprintf("verbs=%v", verbs), func(t *testing.T) {
					backendService := newMockBackendService(t)
					service := newService(t, fakeChecker{allowedVerbs: verbs}, backendService, testReporter, &libevents.DiscardEmitter{}, clockwork.NewFakeClock())
					err := callMethod(t, service, tt.name)
					// expect access denied except with full set of verbs.
					if len(verbs) == len(tt.allowedVerbs) {
						require.False(t, trace.IsAccessDenied(err))
					} else {
						require.True(t, trace.IsAccessDenied(err), "expected access denied for verbs %v, got err=%v", verbs, err)
					}
				})
			}
		})
	}

	// verify that all declared methods have matching test cases
	t.Run("verify coverage", func(t *testing.T) {
		for _, method := range usertasksv1.UserTaskService_ServiceDesc.Methods {
			t.Run(method.MethodName, func(t *testing.T) {
				match := false
				for _, testCase := range testCases {
					match = match || testCase.name == method.MethodName
				}
				require.True(t, match, "method %v without coverage, no matching tests", method.MethodName)
			})
		}
	})
}

func TestEvents(t *testing.T) {
	rwVerbs := []string{types.VerbList, types.VerbCreate, types.VerbRead, types.VerbUpdate, types.VerbDelete}
	testReporter := &mockUsageReporter{}
	auditEventsSink := eventstest.NewChannelEmitter(10)
	fakeClock := clockwork.NewFakeClock()
	backendService := newMockBackendService(t)
	service := newService(t, fakeChecker{allowedVerbs: rwVerbs}, backendService, testReporter, auditEventsSink, fakeClock)
	ctx := context.Background()

	ut1, err := usertasks.NewDiscoverEC2UserTask(&usertasksv1.UserTaskSpec{
		Integration: "my-integration",
		TaskType:    "discover-ec2",
		IssueType:   "ec2-ssm-invocation-failure",
		State:       "OPEN",
		DiscoverEc2: &usertasksv1.DiscoverEC2{
			AccountId: "123456789012",
			Region:    "us-east-1",
			Instances: map[string]*usertasksv1.DiscoverEC2Instance{
				"i-123": {
					InstanceId:      "i-123",
					DiscoveryConfig: "dc01",
					DiscoveryGroup:  "dg01",
				},
			},
		},
	})
	require.NoError(t, err)
	userTaskName := ut1.GetMetadata().GetName()

	createUserTaskResp, err := service.CreateUserTask(ctx, &usertasksv1.CreateUserTaskRequest{UserTask: ut1})
	require.NoError(t, err)
	// Usage reporting happens when user task is created, so we expect to see an event.
	require.Len(t, testReporter.emittedEvents, 1)
	consumeAssertEvent(t, auditEventsSink.C(), auditEventFor(userTaskName, "create", "", ""))
	// LastStateChange is updated.
	require.Equal(t, timestamppb.New(fakeClock.Now()), createUserTaskResp.Status.LastStateChange)

	ut1.Spec.DiscoverEc2.Instances["i-345"] = &usertasksv1.DiscoverEC2Instance{
		InstanceId:      "i-345",
		DiscoveryConfig: "dc01",
		DiscoveryGroup:  "dg01",
	}
	fakeClock.Advance(1 * time.Minute)
	upsertUserTaskResp, err := service.UpsertUserTask(ctx, &usertasksv1.UpsertUserTaskRequest{UserTask: ut1})
	require.NoError(t, err)
	// State was not updated, so usage events must not increase.
	require.Len(t, testReporter.emittedEvents, 1)
	consumeAssertEvent(t, auditEventsSink.C(), auditEventFor(userTaskName, "update", "OPEN", "OPEN"))
	// LastStateChange is not updated.
	require.Equal(t, createUserTaskResp.Status.LastStateChange, upsertUserTaskResp.Status.LastStateChange)

	ut1.Spec.State = "RESOLVED"
	fakeClock.Advance(1 * time.Minute)
	updateUserTaskResp, err := service.UpdateUserTask(ctx, &usertasksv1.UpdateUserTaskRequest{UserTask: ut1})
	require.NoError(t, err)
	// State was updated, so usage events include this new usage report.
	require.Len(t, testReporter.emittedEvents, 2)
	consumeAssertEvent(t, auditEventsSink.C(), auditEventFor(userTaskName, "update", "OPEN", "RESOLVED"))
	// LastStateChange was updated because the state changed.
	require.Equal(t, timestamppb.New(fakeClock.Now()), updateUserTaskResp.Status.LastStateChange)

	_, err = service.DeleteUserTask(ctx, &usertasksv1.DeleteUserTaskRequest{Name: userTaskName})
	require.NoError(t, err)
	// No usage report for deleted resources.
	require.Len(t, testReporter.emittedEvents, 2)
	consumeAssertEvent(t, auditEventsSink.C(), auditEventFor(userTaskName, "delete", "", ""))
}

func auditEventFor(resourceName, eventType, oldState, newState string) any {
	status := apievents.Status{
		Success: true,
	}
	userMetadata := apievents.UserMetadata{
		UserKind: apievents.UserKind_USER_KIND_HUMAN,
	}
	resourceMetadata := apievents.ResourceMetadata{
		Name:      resourceName,
		UpdatedBy: "llama",
	}
	userTaskMetadata := apievents.UserTaskMetadata{
		TaskType:    "discover-ec2",
		IssueType:   "ec2-ssm-invocation-failure",
		Integration: "my-integration",
	}

	var evt any
	switch eventType {
	case "create":
		evt = &apievents.UserTaskCreate{
			Status:           status,
			ResourceMetadata: resourceMetadata,
			UserMetadata:     userMetadata,
			UserTaskMetadata: userTaskMetadata,
			Metadata: apievents.Metadata{
				Type: libevents.UserTaskCreateEvent,
				Code: libevents.UserTaskCreateCode,
			},
		}
	case "update":
		evt = &apievents.UserTaskUpdate{
			Status:           status,
			ResourceMetadata: resourceMetadata,
			UserMetadata:     userMetadata,
			UserTaskMetadata: userTaskMetadata,
			Metadata: apievents.Metadata{
				Type: libevents.UserTaskUpdateEvent,
				Code: libevents.UserTaskUpdateCode,
			},
			CurrentUserTaskState: oldState,
			UpdatedUserTaskState: newState,
		}
	case "delete":
		evt = &apievents.UserTaskDelete{
			Status:           status,
			ResourceMetadata: resourceMetadata,
			UserMetadata:     userMetadata,
			Metadata: apievents.Metadata{
				Type: libevents.UserTaskDeleteEvent,
				Code: libevents.UserTaskDeleteCode,
			},
		}
	}
	return evt
}

func consumeAssertEvent(t *testing.T, q <-chan apievents.AuditEvent, expectedEvent any) {
	select {
	case evt := <-q:
		require.Empty(t, cmp.Diff(expectedEvent, evt, cmpopts.IgnoreFields(apievents.UserMetadata{}, "User")))
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for user task create event")
	}
}

// callMethod calls a method with given name in the UserTask service
func callMethod(t *testing.T, service *Service, method string) error {
	emptyUserTask := &usertasksv1.UserTask{
		Spec: &usertasksv1.UserTaskSpec{},
	}

	for _, desc := range usertasksv1.UserTaskService_ServiceDesc.Methods {
		if desc.MethodName == method {
			_, err := desc.Handler(service, context.Background(), func(arg any) error {
				switch arg := arg.(type) {
				case *usertasksv1.CreateUserTaskRequest:
					arg.UserTask = emptyUserTask
				case *usertasksv1.UpsertUserTaskRequest:
					arg.UserTask = emptyUserTask
				}
				return nil
			}, nil)
			return err
		}
	}
	require.FailNow(t, "method %v not found", method)
	panic("this line should never be reached: FailNow() should interrupt the test")
}

type fakeChecker struct {
	allowedVerbs []string
	services.AccessChecker
}

func (f fakeChecker) CheckAccessToRule(_ services.RuleContext, _ string, resource string, verb string) error {
	if resource == types.KindUserTask {
		if slices.Contains(f.allowedVerbs, verb) {
			return nil
		}
	}

	return trace.AccessDenied("access denied to rule=%v/verb=%v", resource, verb)
}

func newService(t *testing.T, checker services.AccessChecker, backendService BackendService, usageReporter usagereporter.UsageReporter, emitter apievents.Emitter, clock clockwork.Clock) *Service {
	t.Helper()

	authorizer := authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
		user, err := types.NewUser("llama")
		if err != nil {
			return nil, err
		}
		return &authz.Context{
			User:    user,
			Checker: checker,
			Identity: authz.LocalUser{
				Identity: tlsca.Identity{
					Username: user.GetName(),
				},
			},
		}, nil
	})

	service, err := NewService(ServiceConfig{
		Authorizer:    authorizer,
		Backend:       backendService,
		Cache:         backendService,
		UsageReporter: func() usagereporter.UsageReporter { return usageReporter },
		Emitter:       emitter,
		Clock:         clock,
	})
	require.NoError(t, err)
	return service
}

type mockUsageReporter struct {
	emittedEvents []*usagereporter.UserTaskStateEvent
}

func (m *mockUsageReporter) AnonymizeAndSubmit(events ...usagereporter.Anonymizable) {
	for _, e := range events {
		if userTaskEvent, ok := e.(*usagereporter.UserTaskStateEvent); ok {
			m.emittedEvents = append(m.emittedEvents, userTaskEvent)
		}
	}
}

func createDummyUserTask(t *testing.T, integration string) *usertasksv1.UserTask {
	ut, err := usertasks.NewDiscoverEC2UserTask(
		&usertasksv1.UserTaskSpec{
			Integration: integration,
			TaskType:    "discover-ec2",
			IssueType:   "ec2-ssm-invocation-failure",
			State:       "OPEN",
			DiscoverEc2: &usertasksv1.DiscoverEC2{
				AccountId: "123456789012",
				Region:    "us-east-1",
				Instances: map[string]*usertasksv1.DiscoverEC2Instance{
					"i-123": {
						InstanceId:      "i-123",
						DiscoveryConfig: "dc01",
						DiscoveryGroup:  "dg01",
					},
				},
			},
		},
	)
	require.NoError(t, err)
	return ut
}

func TestNotifications(t *testing.T) {
	rwVerbs := []string{types.VerbList, types.VerbCreate, types.VerbRead, types.VerbUpdate, types.VerbDelete}
	backendService := newMockBackendService(t)
	service := newService(t, fakeChecker{allowedVerbs: rwVerbs}, backendService, &mockUsageReporter{}, &libevents.DiscardEmitter{}, clockwork.NewFakeClock())
	ctx := context.Background()

	ut1 := createDummyUserTask(t, "my-integration")
	userTaskName := ut1.GetMetadata().GetName()

	_, err := service.CreateUserTask(ctx, &usertasksv1.CreateUserTaskRequest{UserTask: ut1})
	require.NoError(t, err)
	// A global notification must be created because there's a new UserTask reporting happens when user task is created, so we expect to see an event.
	require.Len(t, backendService.notifications, 1)
	var existingNotification *notificationsv1.GlobalNotification
	for _, v := range backendService.notifications {
		existingNotification = v
	}
	// Integration is updated accordingly to store this notification.
	require.Equal(t, backendService.integrationStatus.PendingUserTasksNotificationID, existingNotification.Metadata.GetName())

	// Updating the User Task to resolved does not trigger a new notification.
	ut1.Spec.State = "RESOLVED"
	_, err = service.UpsertUserTask(ctx, &usertasksv1.UpsertUserTaskRequest{UserTask: ut1})
	require.NoError(t, err)
	require.Equal(t, backendService.integrationStatus.PendingUserTasksNotificationID, existingNotification.Metadata.GetName())

	// But updating it again to OPEN must create a new notification.
	ut1.Spec.State = "OPEN"
	_, err = service.UpsertUserTask(ctx, &usertasksv1.UpsertUserTaskRequest{UserTask: ut1})
	require.NoError(t, err)
	require.NotEqual(t, backendService.integrationStatus.PendingUserTasksNotificationID, existingNotification.Metadata.GetName())

	_, err = service.DeleteUserTask(ctx, &usertasksv1.DeleteUserTaskRequest{Name: userTaskName})
	require.NoError(t, err)
	require.NotEqual(t, backendService.integrationStatus.PendingUserTasksNotificationID, existingNotification.Metadata.GetName())

	// After the notification expires, stale state might exist in the Integration status field.
	// In that case, Deleting the Notification is a no-op and should be handled gracefully.
	delete(backendService.notifications, ut1.GetMetadata().GetName())

	ut2 := createDummyUserTask(t, "integration-for-ut2")
	ut2.Metadata.Expires = &timestamppb.Timestamp{Seconds: 100}
	_, err = service.CreateUserTask(ctx, &usertasksv1.CreateUserTaskRequest{UserTask: ut2})
	require.NoError(t, err)

	// Issuing a new Notification, whose expiration is shorter, should keep the old expiration.
	ut3 := createDummyUserTask(t, "integration-for-ut3")
	ut3.Metadata.Expires = &timestamppb.Timestamp{Seconds: 90}
	_, err = service.CreateUserTask(ctx, &usertasksv1.CreateUserTaskRequest{UserTask: ut3})
	require.NoError(t, err)

	require.Len(t, backendService.notifications, 1)
	for _, v := range backendService.notifications {
		existingNotification = v
		t.Log(v.Metadata.Expires)
	}
	require.Equal(t, int64(100), existingNotification.GetSpec().GetNotification().GetMetadata().GetExpires().GetSeconds())
}

func newMockBackendService(t *testing.T) *mockBackendService {
	b, err := memory.New(memory.Config{})
	require.NoError(t, err)

	backendService, err := local.NewUserTasksService(b)
	require.NoError(t, err)
	return &mockBackendService{
		UserTasks:     backendService,
		notifications: make(map[string]*notificationsv1.GlobalNotification),
	}
}

type mockBackendService struct {
	services.UserTasks

	notifications     map[string]*notificationsv1.GlobalNotification
	integrationStatus types.IntegrationStatusV1
}

func (m *mockBackendService) CreateGlobalNotification(ctx context.Context, globalNotification *notificationsv1.GlobalNotification) (*notificationsv1.GlobalNotification, error) {
	globalNotification.Metadata = &headerv1.Metadata{
		Name: uuid.NewString(),
	}
	m.notifications[globalNotification.GetMetadata().Name] = globalNotification
	return globalNotification, nil
}

func (m *mockBackendService) DeleteGlobalNotification(ctx context.Context, notificationId string) error {
	if _, ok := m.notifications[notificationId]; !ok {
		return trace.NotFound("global notification not found")
	}
	delete(m.notifications, notificationId)
	return nil
}

func (m *mockBackendService) UpdateIntegration(ctx context.Context, integration types.Integration) (types.Integration, error) {
	m.integrationStatus = integration.GetStatus()
	return nil, nil
}

func (m *mockBackendService) GetIntegration(ctx context.Context, name string) (types.Integration, error) {
	return &types.IntegrationV1{
		Status: m.integrationStatus,
	}, nil
}
