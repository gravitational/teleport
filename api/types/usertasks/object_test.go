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

package usertasks_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	usertasksv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/usertasks/v1"
	"github.com/gravitational/teleport/api/types/usertasks"
)

func TestValidateUserTask(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		task    *usertasksv1.UserTask
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:    "NilUserTask",
			task:    nil,
			wantErr: require.Error,
		},
		{
			name: "ValidUserTask",
			task: &usertasksv1.UserTask{
				Kind:    "user_task",
				Version: "v1",
				Metadata: &headerv1.Metadata{
					Name: "test",
				},
				Spec: &usertasksv1.UserTaskSpec{
					Integration: "my-integration",
					TaskType:    "discover-ec2",
					IssueType:   "failed to enroll ec2 instances",
					DiscoverEc2: &usertasksv1.DiscoverEC2{},
				},
			},
			wantErr: require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := usertasks.ValidateUserTask(tt.task)
			tt.wantErr(t, err)
		})
	}
}
