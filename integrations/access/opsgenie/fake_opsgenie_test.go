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

package opsgenie

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
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib/stringset"
)

type FakeOpsgenie struct {
	srv *httptest.Server

	objects sync.Map
	// Alerts
	alertIDCounter uint64
	newAlerts      chan Alert
	alertUpdates   chan Alert
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
	AlertNote
}

func NewFakeOpsgenie(concurrency int) *FakeOpsgenie {
	router := httprouter.New()

	opsgenie := &FakeOpsgenie{
		newAlerts:     make(chan Alert, concurrency),
		alertUpdates:  make(chan Alert, concurrency),
		newAlertNotes: make(chan FakeAlertNote, concurrency*3), // for any alert there could be 1-3 notes
		srv:           httptest.NewServer(router),
	}

	router.POST("/v2/alerts", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)

		var alert Alert
		err := json.NewDecoder(r.Body).Decode(&alert)
		panicIf(err)

		alert.ID = fmt.Sprintf("alert-%v", atomic.AddUint64(&opsgenie.alertIDCounter, 1))
		alert.Status = types.RequestState_PENDING.String()

		opsgenie.StoreAlert(alert)
		opsgenie.newAlerts <- alert

		err = json.NewEncoder(rw).Encode(AlertResult{Alert: alert})
		panicIf(err)
	})
	router.POST("/v2/alerts/:alertID/close", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		alertID := ps.ByName("alertID")

		var body AlertNote
		err := json.NewDecoder(r.Body).Decode(&body)
		panicIf(err)

		note := opsgenie.StoreAlertNote(alertID, AlertNote{Note: body.Note})

		opsgenie.newAlertNotes <- FakeAlertNote{AlertNote: note, AlertID: alertID}
		err = json.NewEncoder(rw).Encode(note)
		panicIf(err)

		alert, found := opsgenie.GetAlert(alertID)
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		alert.Status = "resolved"
		opsgenie.StoreAlert(alert)
		opsgenie.alertUpdates <- alert

	})
	router.POST("/v2/alerts/:alertID/notes", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)

		alertID := ps.ByName("alertID")

		var body AlertNote
		err := json.NewDecoder(r.Body).Decode(&body)
		panicIf(err)

		note := opsgenie.StoreAlertNote(alertID, AlertNote{Note: body.Note})

		opsgenie.newAlertNotes <- FakeAlertNote{AlertNote: note, AlertID: alertID}
		err = json.NewEncoder(rw).Encode(note)
		panicIf(err)

	})
	return opsgenie
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

func (s *FakeOpsgenie) GetResponder(id string) (Responder, bool) {
	if obj, ok := s.objects.Load(id); ok {
		responder, ok := obj.(Responder)
		return responder, ok
	}
	return Responder{}, false
}

func (s *FakeOpsgenie) GetResponderByName(name string) (Responder, bool) {
	if obj, ok := s.objects.Load(fakeResponderByNameKey(strings.ToLower(name))); ok {
		responder, ok := obj.(Responder)
		return responder, ok
	}
	return Responder{}, false
}

func (s *FakeOpsgenie) StoreResponder(responder Responder) Responder {
	byNameKey := fakeResponderByNameKey(strings.ToLower(responder.Name))
	if responder.ID == "" {
		if obj, ok := s.objects.Load(byNameKey); ok {
			responder.ID = obj.(Responder).ID
		} else {
			responder.ID = fmt.Sprintf("responder-%v", atomic.AddUint64(&s.responderIDCounter, 1))
		}
	}
	s.objects.Store(responder.ID, responder)
	s.objects.Store(byNameKey, responder)
	return responder
}

func (s *FakeOpsgenie) GetAlert(id string) (Alert, bool) {
	if obj, ok := s.objects.Load(id); ok {
		alert, ok := obj.(Alert)
		return alert, ok
	}
	return Alert{}, false
}

func (s *FakeOpsgenie) StoreAlert(alert Alert) Alert {
	if alert.ID == "" {
		alert.ID = fmt.Sprintf("alert-%v", atomic.AddUint64(&s.alertIDCounter, 1))
	}
	s.objects.Store(alert.ID, alert)
	return alert
}

func (s *FakeOpsgenie) StoreAlertNote(alertID string, note AlertNote) AlertNote {
	s.objects.Store(alertID+note.Note, note)
	return note
}

func (s *FakeOpsgenie) CheckNewAlert(ctx context.Context) (Alert, error) {
	select {
	case alert := <-s.newAlerts:
		return alert, nil
	case <-ctx.Done():
		return Alert{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakeOpsgenie) CheckAlertUpdate(ctx context.Context) (Alert, error) {
	select {
	case alert := <-s.alertUpdates:
		return alert, nil
	case <-ctx.Done():
		return Alert{}, trace.Wrap(ctx.Err())
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

func panicIf(err error) {
	if err != nil {
		log.Panicf("%v at %v", err, string(debug.Stack()))
	}
}
