/*
Copyright 2025 Gravitational, Inc.

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

package notifications

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	notificationsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/notifications/v1"
	"github.com/gravitational/teleport/api/types"
)

// NewPendingUserTasksIntegrationNotification creates a new GlobalNotification for warning the users about pending UserTasks related to an integration.
func NewPendingUserTasksIntegrationNotification(integrationName string, expires time.Time) *notificationsv1.GlobalNotification {
	return &notificationsv1.GlobalNotification{
		Spec: &notificationsv1.GlobalNotificationSpec{
			Matcher: &notificationsv1.GlobalNotificationSpec_ByPermissions{
				ByPermissions: &notificationsv1.ByPermissions{
					RoleConditions: []*types.RoleConditions{{
						Rules: []types.Rule{{
							Resources: []string{types.KindIntegration},
							Verbs:     []string{types.VerbList, types.VerbRead},
						}},
					}},
				},
			},
			Notification: &notificationsv1.Notification{
				Spec:    &notificationsv1.NotificationSpec{},
				SubKind: types.NotificationPendingUserTaskIntegrationSubKind,
				Metadata: &headerv1.Metadata{
					Labels: map[string]string{
						types.NotificationTitleLabel:       "Your integration needs attention.",
						types.NotificationIntegrationLabel: integrationName,
					},
					Expires: timestamppb.New(expires),
				},
			},
		},
	}
}
