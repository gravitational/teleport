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

package maintenance

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_kubeScheduleRepr_isValid(t *testing.T) {
	clock := clockwork.NewFakeClock()
	now := clock.Now()

	tests := []struct {
		name    string
		windows []windowRepr
		want    bool
	}{
		{
			name: "one future window",
			windows: []windowRepr{
				{Start: now.Add(time.Hour), Stop: now.Add(2 * time.Hour)},
			},
			want: true,
		},
		{
			name: "past and future windows",
			windows: []windowRepr{
				{Start: now.Add(-2 * time.Hour), Stop: now.Add(-time.Hour)},
				{Start: now.Add(time.Hour), Stop: now.Add(2 * time.Hour)},
			},
			want: true,
		},
		{
			name: "one active window",
			windows: []windowRepr{
				{Start: now.Add(-time.Hour), Stop: now.Add(time.Hour)},
			},
			want: true,
		},
		{
			name: "only past window",
			windows: []windowRepr{
				{Start: now.Add(-2 * time.Hour), Stop: now.Add(-time.Hour)},
			},
			want: false,
		},
		{
			name:    "no window",
			windows: []windowRepr{},
			want:    false,
		},
		{
			name: "one broken window",
			windows: []windowRepr{
				{Start: now.Add(time.Hour), Stop: now.Add(-time.Hour)},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := kubeScheduleRepr{
				Windows: tt.windows,
			}
			require.Equal(t, tt.want, s.isValid(now))
		})
	}
}

func Test_windowRepr_inWindow(t *testing.T) {
	clock := clockwork.NewFakeClock()
	now := clock.Now()

	tests := []struct {
		name  string
		start time.Time
		stop  time.Time
		want  bool
	}{
		{
			name:  "before window",
			start: now.Add(time.Hour),
			stop:  now.Add(2 * time.Hour),
			want:  false,
		},
		{
			name:  "after window",
			start: now.Add(-2 * time.Hour),
			stop:  now.Add(-time.Hour),
			want:  false,
		},
		{
			name:  "in window",
			start: now.Add(-time.Hour),
			stop:  now.Add(time.Hour),
			want:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := windowRepr{Start: tt.start, Stop: tt.stop}
			require.Equal(t, tt.want, w.inWindow(now))
		})
	}
}

func TestWindowTrigger_CanStart(t *testing.T) {
	// Test setup: generating and loading fixtures
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	now := clock.Now()
	namespace := "bar"

	invalidSchedule, err := json.Marshal(kubeScheduleRepr{Windows: []windowRepr{
		{Start: now.Add(-time.Hour), Stop: now.Add(time.Hour)},
		{Start: now.Add(time.Hour), Stop: now.Add(-time.Hour)},
	}})
	require.NoError(t, err)
	futureSchedule, err := json.Marshal(kubeScheduleRepr{Windows: []windowRepr{
		{Start: now.Add(time.Hour), Stop: now.Add(2 * time.Hour)},
	}})
	require.NoError(t, err)
	activeSchedule, err := json.Marshal(kubeScheduleRepr{Windows: []windowRepr{
		{Start: now.Add(-time.Hour), Stop: now.Add(time.Hour)},
	}})
	require.NoError(t, err)

	fixtures := &v1.SecretList{Items: []v1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "no-key-shared-state", Namespace: namespace},
			Data:       map[string][]byte{"foo": []byte("bar")},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "invalid-json-shared-state", Namespace: namespace},
			Data:       map[string][]byte{maintenanceScheduleKeyName: []byte(`{"foo": "bar"}`)},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "invalid-schedule-shared-state", Namespace: namespace},
			Data:       map[string][]byte{maintenanceScheduleKeyName: invalidSchedule},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "in-maintenance-shared-state", Namespace: namespace},
			Data:       map[string][]byte{maintenanceScheduleKeyName: activeSchedule},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "not-in-maintenance-shared-state", Namespace: namespace},
			Data:       map[string][]byte{maintenanceScheduleKeyName: futureSchedule},
		},
	}}

	clientBuilder := fake.NewClientBuilder()
	clientBuilder.WithLists(fixtures)
	client := clientBuilder.Build()

	tests := []struct {
		name      string
		object    kclient.Object
		want      bool
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "no secret",
			object:    &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "not-found", Namespace: namespace}},
			assertErr: require.Error,
		},
		{
			name:      "secret no key",
			object:    &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "no-key", Namespace: namespace}},
			assertErr: require.Error,
		},
		{
			name:      "secret invalid JSON",
			object:    &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "invalid-json", Namespace: namespace}},
			assertErr: require.Error,
		},
		{
			name:      "secret invalid schedule",
			object:    &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "invalid-schedule", Namespace: namespace}},
			assertErr: require.Error,
		},
		{
			name:      "in maintenance window",
			object:    &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "in-maintenance", Namespace: namespace}},
			want:      true,
			assertErr: require.NoError,
		},
		{
			name:      "not in maintenance window",
			object:    &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "not-in-maintenance", Namespace: namespace}},
			want:      false,
			assertErr: require.NoError,
		},
	}
	// Doing the real test
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := windowTrigger{
				Client: client,
				clock:  clock,
			}
			got, err := w.CanStart(ctx, tt.object)
			tt.assertErr(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
