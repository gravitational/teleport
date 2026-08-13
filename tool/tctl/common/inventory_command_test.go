/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestFormatAuditQueue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status *types.AuditQueueStatus
		want   string
	}{
		{
			name:   "not reported",
			status: nil,
			want:   "",
		},
		{
			name:   "empty queue",
			status: &types.AuditQueueStatus{},
			want:   "0",
		},
		{
			name:   "pending only",
			status: &types.AuditQueueStatus{PendingCount: 12},
			want:   "12",
		},
		{
			name: "pending with age",
			status: &types.AuditQueueStatus{
				PendingCount:            12,
				OldestPendingAgeSeconds: int64((5 * time.Minute).Seconds()),
			},
			want: "12 (oldest 5 minutes)",
		},
		{
			name: "young ages are hidden",
			status: &types.AuditQueueStatus{
				PendingCount:               12,
				DeadLetterCount:            3,
				OldestPendingAgeSeconds:    int64((4 * time.Minute).Seconds()),
				OldestDeadLetterAgeSeconds: int64((2 * time.Minute).Seconds()),
			},
			want: "12 (3 DL)",
		},
		{
			name: "dead letter with age",
			status: &types.AuditQueueStatus{
				PendingCount:               2,
				DeadLetterCount:            3,
				OldestDeadLetterAgeSeconds: int64((48 * time.Hour).Seconds()),
			},
			want: "2 (3 DL, oldest 2 days)",
		},
		{
			name: "dead letter without age",
			status: &types.AuditQueueStatus{
				PendingCount:    0,
				DeadLetterCount: 3,
			},
			want: "0 (3 DL)",
		},
		{
			name: "all parts",
			status: &types.AuditQueueStatus{
				PendingCount:               12,
				DeadLetterCount:            3,
				CorruptCount:               1,
				OldestPendingAgeSeconds:    int64(time.Hour.Seconds()),
				OldestDeadLetterAgeSeconds: int64((30 * 24 * time.Hour).Seconds()),
			},
			want: "12 (oldest 1 hour) (3 DL, oldest 1 month) (1 corrupt)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, formatAuditQueue(tc.status))
		})
	}
}
