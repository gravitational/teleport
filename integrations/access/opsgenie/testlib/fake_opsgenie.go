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
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/opsgenie"
	"github.com/gravitational/teleport/integrations/lib/stringset"
)

type FakeOpsgenie struct {
	srv *httptest.Server

	objects sync.Map
	// Alerts
	alertIDCounter uint64
	newAlerts      chan opsgenie.Alert
	alertUpdates   chan opsgenie.Alert
	// Alert notes
	newAlertNotes chan FakeAlertNote
	// Responders
	responderIDCounter uint64
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

type fakeResponderByNameKey string

type FakeAlertNote struct {
	AlertID string
	opsgenie.AlertNote
}

func NewFakeOpsgenie(concurrency int) *FakeOpsgenie {
	router := httprouter.New()

	mock := &FakeOpsgenie{
		newAlerts:     make(chan opsgenie.Alert, concurrency),
		alertUpdates:  make(chan opsgenie.Alert, concurrency),
		newAlertNotes: make(chan FakeAlertNote, concurrency*3), // for any alert there could be 1-3 notes
		srv:           httptest.NewServer(router),
	}

	router.POST("/v2/alerts", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)

		var alert opsgenie.Alert
		err := json.NewDecoder(r.Body).Decode(&alert)
		panicIf(err)

		alert.ID = fmt.Sprintf("alert-%v", atomic.AddUint64(&mock.alertIDCounter, 1))
		alert.Status = types.RequestState_PENDING.String()

		mock.StoreAlert(alert)
		mock.newAlerts <- alert

		err = json.NewEncoder(rw).Encode(opsgenie.CreateAlertResult{RequestID: alert.ID})
		panicIf(err)
	})
	router.GET("/v2/alerts/requests/:requestID", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)

		requestID := ps.ByName("requestID")
		err := json.NewEncoder(rw).Encode(opsgenie.GetAlertRequestResult{
			Data: struct {
				AlertID string `json:"alertId"`
			}{
				AlertID: requestID,
			},
		})
		panicIf(err)
	})
	router.POST("/v2/alerts/:alertID/close", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		alertID := ps.ByName("alertID")

		var body opsgenie.AlertNote
		err := json.NewDecoder(r.Body).Decode(&body)
		panicIf(err)

		note := mock.StoreAlertNote(alertID, opsgenie.AlertNote{Note: body.Note})

		mock.newAlertNotes <- FakeAlertNote{AlertNote: note, AlertID: alertID}
		err = json.NewEncoder(rw).Encode(note)
		panicIf(err)

		alert, found := mock.GetAlert(alertID)
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		alert.Status = "resolved"
		mock.StoreAlert(alert)
		mock.alertUpdates <- alert

	})
	router.POST("/v2/alerts/:alertID/notes", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)

		alertID := ps.ByName("alertID")

		var body opsgenie.AlertNote
		err := json.NewDecoder(r.Body).Decode(&body)
		panicIf(err)

		note := mock.StoreAlertNote(alertID, opsgenie.AlertNote{Note: body.Note})

		mock.newAlertNotes <- FakeAlertNote{AlertNote: note, AlertID: alertID}
		err = json.NewEncoder(rw).Encode(note)
		panicIf(err)

	})
	router.GET("/v2/schedules/:scheduleName/on-calls", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		scheduleName := ps.ByName("scheduleName")

		// Check if exists
		_, ok := mock.GetSchedule(scheduleName)
		if !ok {
			rw.WriteHeader(http.StatusNotFound)
			return
		}

		emails := mock.GetOnCallEmailsForSchedule(scheduleName)

		response := opsgenie.RespondersResult{
			Data: struct {
				OnCallRecipients []string `json:"onCallRecipients,omitempty"`
			}(
				struct {
					OnCallRecipients []string
				}{
					OnCallRecipients: emails,
				},
			),
		}

		rw.WriteHeader(http.StatusOK)
		err := json.NewEncoder(rw).Encode(response)
		panicIf(err)
	})
	router.GET("/v2/heartbeats/teleport-access-heartbeat/ping", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.WriteHeader(http.StatusOK)
	})
	return mock
}

func (s *FakeOpsgenie) URL() string {
	return s.srv.URL
}

func (s *FakeOpsgenie) Close() {
	s.srv.Close()
	close(s.newAlerts)
	close(s.alertUpdates)
	close(s.newAlertNotes)
}

func (s *FakeOpsgenie) GetResponder(id string) (opsgenie.Responder, bool) {
	if obj, ok := s.objects.Load(id); ok {
		responder, ok := obj.(opsgenie.Responder)
		return responder, ok
	}
	return opsgenie.Responder{}, false
}

func (s *FakeOpsgenie) GetResponderByName(name string) (opsgenie.Responder, bool) {
	if obj, ok := s.objects.Load(fakeResponderByNameKey(strings.ToLower(name))); ok {
		responder, ok := obj.(opsgenie.Responder)
		return responder, ok
	}
	return opsgenie.Responder{}, false
}

func (s *FakeOpsgenie) StoreResponder(responder opsgenie.Responder) opsgenie.Responder {
	byNameKey := fakeResponderByNameKey(strings.ToLower(responder.Name))
	if responder.ID == "" {
		if obj, ok := s.objects.Load(byNameKey); ok {
			responder.ID = obj.(opsgenie.Responder).ID
		} else {
			responder.ID = fmt.Sprintf("responder-%v", atomic.AddUint64(&s.responderIDCounter, 1))
		}
	}
	s.objects.Store(responder.ID, responder)
	s.objects.Store(byNameKey, responder)
	return responder
}

func (s *FakeOpsgenie) GetAlert(id string) (opsgenie.Alert, bool) {
	if obj, ok := s.objects.Load(id); ok {
		alert, ok := obj.(opsgenie.Alert)
		return alert, ok
	}
	return opsgenie.Alert{}, false
}

func (s *FakeOpsgenie) StoreAlert(alert opsgenie.Alert) opsgenie.Alert {
	if alert.ID == "" {
		alert.ID = fmt.Sprintf("alert-%v", atomic.AddUint64(&s.alertIDCounter, 1))
	}
	s.objects.Store(alert.ID, alert)
	return alert
}

func (s *FakeOpsgenie) StoreAlertNote(alertID string, note opsgenie.AlertNote) opsgenie.AlertNote {
	s.objects.Store(alertID+note.Note, note)
	return note
}

func (s *FakeOpsgenie) CheckNewAlert(ctx context.Context) (opsgenie.Alert, error) {
	select {
	case alert := <-s.newAlerts:
		return alert, nil
	case <-ctx.Done():
		return opsgenie.Alert{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakeOpsgenie) CheckAlertUpdate(ctx context.Context) (opsgenie.Alert, error) {
	select {
	case alert := <-s.alertUpdates:
		return alert, nil
	case <-ctx.Done():
		return opsgenie.Alert{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakeOpsgenie) CheckNewAlertNote(ctx context.Context) (FakeAlertNote, error) {
	select {
	case note := <-s.newAlertNotes:
		return note, nil
	case <-ctx.Done():
		return FakeAlertNote{}, trace.Wrap(ctx.Err())
	}
}

// StoreSchedule upserts a schedule. To simplify the implementation, the schedule
// is not given a UUID, we use its name. This is possible because we get the on-call
// list in Client.GetOnCall() by passing the param "scheduleIdentifierType": "name".
// The function also creates a responder for the schedule and returns it.
// The schedule can then be directly notified as a responder, or queried for
// on-call users as a schedule.
func (s *FakeOpsgenie) StoreSchedule(scheduleName string, responders ...opsgenie.Responder) opsgenie.Responder {
	key := fmt.Sprintf("schedule-%s", scheduleName)
	s.objects.Store(key, responders)
	responder := opsgenie.Responder{
		Name: scheduleName,
		Type: opsgenie.ResponderTypeSchedule,
	}
	responder = s.StoreResponder(responder)
	return responder
}

// GetSchedule gets a schedule.
func (s *FakeOpsgenie) GetSchedule(scheduleName string) ([]opsgenie.Responder, bool) {
	key := fmt.Sprintf("schedule-%s", scheduleName)
	value, ok := s.objects.Load(key)
	if !ok {
		return nil, false
	}
	responders, ok := value.([]opsgenie.Responder)
	if !ok {
		panic("cannot cast schedule object as a responder slice")
	}
	return responders, true
}

func panicIf(err error) {
	if err != nil {
		panic(fmt.Sprintf("%v at %v", err, string(debug.Stack())))
	}
}

func (s *FakeOpsgenie) GetOnCallEmailsForSchedule(scheduleName string) []string {
	var emails []string
	responders, ok := s.GetSchedule(scheduleName)
	if !ok {
		return nil
	}
	for _, responder := range responders {
		switch responder.Type {
		case opsgenie.ResponderTypeSchedule:
			emails = append(emails, s.GetOnCallEmailsForSchedule(responder.Name)...)
		case opsgenie.ResponderTypeUser:
			// If the responder is a user, we return its email
			emails = append(emails, responder.Name)
		default:
			// We don't implement "team" and "escalation" responder types because
			// we don't test those yet.
		}

	}

	return emails
}
