/*
Copyright 2023 Gravitational, Inc.

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

package servicenow

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

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/integrations/lib/stringset"
)

type FakeServiceNow struct {
	srv *httptest.Server

	objects sync.Map
	// Incidents
	incidentIDCounter uint64
	newIncidents      chan Incident
	incidentUpdates   chan Incident
	// Incident notes
	newIncidentNotes chan string
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

type FakeIncident struct {
	IncidentID string
	Incident
}

func NewFakeServiceNow(concurrency int) *FakeServiceNow {
	router := httprouter.New()

	serviceNow := &FakeServiceNow{
		newIncidents:     make(chan Incident, concurrency),
		incidentUpdates:  make(chan Incident, concurrency),
		newIncidentNotes: make(chan string, concurrency*3), // for any incident there could be 1-3 notes
		srv:              httptest.NewServer(router),
	}

	router.POST("/api/now/v1/table/incident", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)

		var incident Incident
		err := json.NewDecoder(r.Body).Decode(&incident)
		panicIf(err)

		incident.IncidentID = fmt.Sprintf("incident-%v", atomic.AddUint64(&serviceNow.incidentIDCounter, 1))

		serviceNow.StoreIncident(incident)
		serviceNow.newIncidents <- incident

		err = json.NewEncoder(rw).Encode(incidentResult{Result: incident})
		panicIf(err)
	})
	router.PATCH("/api/now/v1/table/incident/:incidentID/", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		incidentID := ps.ByName("incidentID")

		var body Incident
		err := json.NewDecoder(r.Body).Decode(&body)
		panicIf(err)

		incident, found := serviceNow.GetIncident(incidentID)
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
		serviceNow.StoreIncident(incident)
		serviceNow.incidentUpdates <- incident
	})
	return serviceNow
}

func (s *FakeServiceNow) URL() string {
	return s.srv.URL
}

func (s *FakeServiceNow) Close() {
	s.srv.Close()
	close(s.newIncidents)
	close(s.incidentUpdates)
}

func (s *FakeServiceNow) GetIncident(id string) (Incident, bool) {
	if obj, ok := s.objects.Load(id); ok {
		incident, ok := obj.(Incident)
		return incident, ok
	}
	return Incident{}, false
}

func (s *FakeServiceNow) StoreIncident(incident Incident) Incident {
	if incident.IncidentID == "" {
		incident.IncidentID = fmt.Sprintf("incident-%v", atomic.AddUint64(&s.incidentIDCounter, 1))
	}
	s.objects.Store(incident.IncidentID, incident)
	return incident
}

func (s *FakeServiceNow) CheckNewIncident(ctx context.Context) (Incident, error) {
	select {
	case incident := <-s.newIncidents:
		return incident, nil
	case <-ctx.Done():
		return Incident{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakeServiceNow) CheckIncidentUpdate(ctx context.Context) (Incident, error) {
	select {
	case incident := <-s.incidentUpdates:
		return incident, nil
	case <-ctx.Done():
		return Incident{}, trace.Wrap(ctx.Err())
	}
}

func (s FakeServiceNow) StoreResponder(ctx context.Context, responderID string) string {
	s.objects.Store(responderID, responderID)
	return responderID
}

func panicIf(err error) {
	if err != nil {
		log.Panicf("%v at %v", err, string(debug.Stack()))
	}
}
