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

package testlib

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime/debug"
	"sync"
	"time"

	"github.com/gravitational/teleport/integrations/access/incidentio"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/integrations/lib/stringset"
)

type FakeIncident struct {
	srv *httptest.Server

	objects sync.Map
	// Alerts
	alertIDCounter uint64
	newAlerts      chan incidentio.AlertBody
	alertUpdates   chan incidentio.AlertBody
}

type QueryValues url.Values

func (q QueryValues) GetAsSet(name string) stringset.StringSet {
	values := q[name]
	result := stringset.NewWithCap(len(values))
	for _, v := range values {
		if v != "" {
			result[v] = struct{}{}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func NewFakeIncident(concurrency int) *FakeIncident {
	router := httprouter.New()

	mock := &FakeIncident{
		newAlerts:    make(chan incidentio.AlertBody, concurrency),
		alertUpdates: make(chan incidentio.AlertBody, concurrency),
		srv:          httptest.NewServer(router),
	}

	router.GET("/v2/schedules/:scheduleID", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		scheduleID := ps.ByName("scheduleID")

		// Check if exists
		_, ok := mock.GetSchedule(scheduleID)
		if !ok {
			rw.WriteHeader(http.StatusNotFound)
			return
		}

		emails := mock.GetOnCallEmailsForSchedule(scheduleID)

		response := incidentio.ScheduleResult{
			Annotations: map[string]string{},
			Config: incidentio.ScheduleConfig{
				Rotations: nil,
			},
			CreatedAt: time.Time{},
			CurrentShifts: []incidentio.CurrentShift{
				{
					EndAt:       time.Time{},
					EntryID:     "someEntryID",
					Fingerprint: "someFingerprint",
					LayerID:     "someLayerID",
					RotationID:  "someRotationID",
					StartAt:     time.Time{},
					User: &incidentio.User{
						Email: emails[0],
					},
				},
			},
			HolidaysPublicConfig: incidentio.HolidaysPublicConfig{
				CountryCodes: nil,
			},
			ID:        "someSchedule",
			Name:      "Schedule",
			Timezone:  "UTC",
			UpdatedAt: time.Time{},
		}

		rw.WriteHeader(http.StatusOK)
		err := json.NewEncoder(rw).Encode(response)
		panicIf(err)
	})
	router.GET("/v2/schedules", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.WriteHeader(http.StatusOK)
	})
	router.POST("/v2/alert_events/http/someRequestID", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)

		var alert incidentio.AlertBody
		err := json.NewDecoder(r.Body).Decode(&alert)
		panicIf(err)

		if alert.Status == "firing" {
			mock.newAlerts <- alert
		} else if alert.Status == "resolved" {
			mock.alertUpdates <- alert
		} else {
			panic("unsupported alert status")
		}
		mock.StoreAlert(alert)

		err = json.NewEncoder(rw).Encode(alert)
		panicIf(err)
	})
	return mock
}

func (s *FakeIncident) URL() string {
	return s.srv.URL
}

func (s *FakeIncident) Close() {
	s.srv.Close()
	close(s.newAlerts)
	close(s.alertUpdates)
}

func (s *FakeIncident) GetAlert(id string) (incidentio.AlertBody, bool) {
	if obj, ok := s.objects.Load(id); ok {
		alert, ok := obj.(incidentio.AlertBody)
		return alert, ok
	}
	return incidentio.AlertBody{}, false
}

func (s *FakeIncident) StoreAlert(alert incidentio.AlertBody) incidentio.AlertBody {
	s.objects.Store(alert.DeduplicationKey, alert)
	return alert
}

func (s *FakeIncident) CheckNewAlert(ctx context.Context) (incidentio.AlertBody, error) {
	select {
	case alert := <-s.newAlerts:
		return alert, nil
	case <-ctx.Done():
		return incidentio.AlertBody{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakeIncident) CheckAlertUpdate(ctx context.Context) (incidentio.AlertBody, error) {
	select {
	case alert := <-s.alertUpdates:
		return alert, nil
	case <-ctx.Done():
		return incidentio.AlertBody{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakeIncident) StoreSchedule(scheduleID string, schedule incidentio.ScheduleResult) incidentio.ScheduleResult {
	key := fmt.Sprintf("schedule-%s", scheduleID)
	s.objects.Store(key, schedule)
	return schedule
}

func (s *FakeIncident) GetSchedule(scheduleID string) (*incidentio.ScheduleResult, bool) {
	key := fmt.Sprintf("schedule-%s", scheduleID)
	value, ok := s.objects.Load(key)
	if !ok {
		return nil, false
	}
	schedule, ok := value.(incidentio.ScheduleResult)
	if !ok {
		panic("cannot cast to schedule")
	}
	return &schedule, true
}

func panicIf(err error) {
	if err != nil {
		log.Panicf("%v at %v", err, string(debug.Stack()))
	}
}

func (s *FakeIncident) GetOnCallEmailsForSchedule(scheduleID string) []string {
	var emails []string
	schedule, ok := s.GetSchedule(scheduleID)
	if !ok {
		return nil
	}
	for _, shift := range schedule.CurrentShifts {
		if shift.User != nil {
			emails = append(emails, shift.User.Email)
		}
	}

	return emails
}
