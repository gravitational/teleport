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
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/integrations/access/servicenow"
	"github.com/gravitational/teleport/integrations/lib/stringset"
)

// FakeServiceNow implements a mock ServiceNow for testing purposes.
// The on_call_rota API is not publicly documented, but you can access
// swagger-like interface by registering a dev SNow account and requesting a dev
// instance.
// When the dev instance is created, you can open the "ALL" tab and search for
// the REST API explorer.
type FakeServiceNow struct {
	srv *httptest.Server

	objects sync.Map

	// Incidents
	incidentIDCounter uint64
	newIncidents      chan servicenow.Incident
	incidentUpdates   chan servicenow.Incident
	// Incident notes
	newIncidentNotes chan string
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

type FakeIncident struct {
	IncidentID string
	servicenow.Incident
}

func NewFakeServiceNow(concurrency int) *FakeServiceNow {
	router := httprouter.New()

	mock := &FakeServiceNow{
		newIncidents:     make(chan servicenow.Incident, concurrency),
		incidentUpdates:  make(chan servicenow.Incident, concurrency),
		newIncidentNotes: make(chan string, concurrency*3), // for any incident there could be 1-3 notes
		srv:              httptest.NewServer(router),
	}
	router.GET("/api/now/table/incident", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {})
	router.POST("/api/now/v1/table/incident", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)

		var incident servicenow.Incident
		err := json.NewDecoder(r.Body).Decode(&incident)
		panicIf(err)

		incident.IncidentID = fmt.Sprintf("incident-%v", atomic.AddUint64(&mock.incidentIDCounter, 1))

		mock.StoreIncident(incident)
		mock.newIncidents <- incident

		err = json.NewEncoder(rw).Encode(servicenow.IncidentResult{Result: struct {
			IncidentID       string `json:"sys_id,omitempty"`
			ShortDescription string `json:"short_description,omitempty"`
			Description      string `json:"description,omitempty"`
			CloseCode        string `json:"close_code,omitempty"`
			CloseNotes       string `json:"close_notes,omitempty"`
			IncidentState    string `json:"incident_state,omitempty"`
			WorkNotes        string `json:"work_notes,omitempty"`
		}{
			IncidentID:       incident.IncidentID,
			ShortDescription: incident.ShortDescription,
			Description:      incident.Description,
			CloseCode:        incident.CloseCode,
			CloseNotes:       incident.CloseNotes,
			IncidentState:    incident.IncidentState,
			WorkNotes:        incident.WorkNotes,
		}})
		panicIf(err)
	})
	router.PATCH("/api/now/v1/table/incident/:incidentID/", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		incidentID := ps.ByName("incidentID")

		var body servicenow.Incident
		err := json.NewDecoder(r.Body).Decode(&body)
		panicIf(err)

		incident, found := mock.GetIncident(incidentID)
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		if body.WorkNotes != "" {
			incident.WorkNotes = body.WorkNotes
		}
		if body.CloseNotes != "" {
			incident.CloseNotes = body.CloseNotes
		}
		if body.CloseCode != "" {
			incident.CloseCode = body.CloseCode
		}
		if body.IncidentState != "" {
			incident.IncidentState = body.IncidentState
		}
		mock.StoreIncident(incident)
		mock.incidentUpdates <- incident
	})
	router.GET("/api/now/on_call_rota/whoisoncall", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// It looks like this could support multiple rotation IDs
		// but there's no documentation as to how it behaves (is it a union,
		// intersection, something else?)
		rotation := r.URL.Query().Get("rota_ids")

		// rotation must be specified
		if rotation == "" {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
		userIDs := mock.getOnCall(rotation)

		var result servicenow.OnCallResult
		for _, userID := range userIDs {
			result.Result = append(result.Result, struct {
				UserID string `json:"userId"`
			}{
				UserID: userID,
			})
		}
		rw.Header().Add("Content-Type", "application/json")

		err := json.NewEncoder(rw).Encode(result)
		panicIf(err)
	})
	router.GET("/api/now/table/sys_user/:UserID", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		userID := ps.ByName("UserID")
		// user id must be specified
		if userID == "" {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}
		userName := mock.getUser(userID)
		if userName == "" {
			rw.WriteHeader(http.StatusNotFound)
			return
		}

		rw.Header().Add("Content-Type", "application/json")
		err := json.NewEncoder(rw).Encode(servicenow.UserResult{Result: struct {
			UserName string `json:"user_name"`
		}{
			UserName: userName,
		}})
		panicIf(err)
	})
	return mock
}

func (s *FakeServiceNow) URL() string {
	return s.srv.URL
}

func (s *FakeServiceNow) Close() {
	s.srv.Close()
	close(s.newIncidents)
	close(s.incidentUpdates)
}

func (s *FakeServiceNow) GetIncident(id string) (servicenow.Incident, bool) {
	if obj, ok := s.objects.Load(id); ok {
		incident, ok := obj.(servicenow.Incident)
		return incident, ok
	}
	return servicenow.Incident{}, false
}

func (s *FakeServiceNow) StoreIncident(incident servicenow.Incident) servicenow.Incident {
	if incident.IncidentID == "" {
		incident.IncidentID = fmt.Sprintf("incident-%v", atomic.AddUint64(&s.incidentIDCounter, 1))
	}
	s.objects.Store(incident.IncidentID, incident)
	return incident
}

func (s *FakeServiceNow) CheckNewIncident(ctx context.Context) (servicenow.Incident, error) {
	select {
	case incident := <-s.newIncidents:
		return incident, nil
	case <-ctx.Done():
		return servicenow.Incident{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakeServiceNow) CheckIncidentUpdate(ctx context.Context) (servicenow.Incident, error) {
	select {
	case incident := <-s.incidentUpdates:
		return incident, nil
	case <-ctx.Done():
		return servicenow.Incident{}, trace.Wrap(ctx.Err())
	}
}

// StoreOnCall allows creating a fake on-call rotation (called shift in
// SevriceNow UI) and set users on-call in this rotation.
func (s *FakeServiceNow) StoreOnCall(rotaID string, userIDs []string) {
	key := fmt.Sprintf("rota-%s", rotaID)
	s.objects.Store(key, userIDs)
}

// StoreUser creates a fake ServiceNow user and returns its userID.
func (s *FakeServiceNow) StoreUser(userName string) string {
	userID := uuid.New()
	key := fmt.Sprintf("user-%s", userID)
	s.objects.Store(key, userName)
	return userID.String()
}

func (s *FakeServiceNow) getUser(userID string) string {
	key := fmt.Sprintf("user-%s", userID)
	value, ok := s.objects.Load(key)
	if !ok {
		return ""
	}
	userName, ok := value.(string)
	if !ok {
		panic(trace.BadParameter("wrong key value, the user name should be a string"))
	}
	return userName
}

func (s *FakeServiceNow) getOnCall(rotationName string) []string {
	key := fmt.Sprintf("rota-%s", rotationName)
	value, ok := s.objects.Load(key)
	if !ok {
		return nil
	}
	userID, ok := value.([]string)
	if !ok {
		panic(trace.BadParameter("wrong key value, the user ids should be a string slice"))
	}
	return userID
}

func panicIf(err error) {
	if err != nil {
		panic(fmt.Sprintf("%v at %v", err, string(debug.Stack())))
	}
}
