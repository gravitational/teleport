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

	"github.com/gravitational/teleport/integrations/access/pagerduty"
	"github.com/gravitational/teleport/integrations/lib/stringset"
)

type FakePagerduty struct {
	srv *httptest.Server

	objects sync.Map
	// Extensions
	extensionIDCounter uint64
	newExtensions      chan pagerduty.Extension
	// Inicidents
	incidentIDCounter uint64
	newIncidents      chan pagerduty.Incident
	incidentUpdates   chan pagerduty.Incident
	// Incident notes
	newIncidentNotes      chan FakeIncidentNote
	incidentNoteIDCounter uint64
	// Services
	serviceIDCounter uint64
	// Users
	userIDCounter uint64
	// OnCalls
	onCallIDCounter uint64
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

type fakeServiceByNameKey string
type fakeUserByEmailKey string

type FakeIncidentNote struct {
	IncidentID string
	pagerduty.IncidentNote
}

func NewFakePagerduty(concurrency int) *FakePagerduty {
	router := httprouter.New()

	mock := &FakePagerduty{
		newExtensions:    make(chan pagerduty.Extension, concurrency),
		newIncidents:     make(chan pagerduty.Incident, concurrency),
		incidentUpdates:  make(chan pagerduty.Incident, concurrency),
		newIncidentNotes: make(chan FakeIncidentNote, concurrency*3), // for any incident there could be 1-3 notes
		srv:              httptest.NewServer(router),
	}

	router.GET("/services/:id", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		id := ps.ByName("id")
		service, found := mock.GetService(id)
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(pagerduty.ErrorResult{Message: "Service not found"})
			panicIf(err)
			return
		}

		err := json.NewEncoder(rw).Encode(pagerduty.ServiceResult{Service: service})
		panicIf(err)
	})
	router.GET("/services", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		var services []pagerduty.Service
		if query := r.URL.Query().Get("query"); query != "" {
			if service, ok := mock.GetServiceByName(query); ok {
				services = append(services, service)
			}
		} else {
			mock.objects.Range(func(key, value interface{}) bool {
				if key, ok := key.(string); !ok || !strings.HasPrefix(key, "service-") {
					return true
				}
				if service, ok := value.(pagerduty.Service); ok {
					services = append(services, service)
				}
				return true
			})
		}

		err := json.NewEncoder(rw).Encode(pagerduty.ListServicesResult{Services: services})
		panicIf(err)
	})
	router.GET("/extension_schemas", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		resp := pagerduty.ListExtensionSchemasResult{
			PaginationResult: pagerduty.PaginationResult{
				More:  false,
				Total: 1,
			},
			ExtensionSchemas: []pagerduty.ExtensionSchema{
				{
					ID:  "11",
					Key: "custom_webhook",
				},
			},
		}
		err := json.NewEncoder(rw).Encode(resp)
		panicIf(err)
	})
	router.GET("/extensions", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		extensions := []pagerduty.Extension{}
		mock.objects.Range(func(key, value interface{}) bool {
			if extension, ok := value.(pagerduty.Extension); ok {
				extensions = append(extensions, extension)
			}
			return true
		})
		resp := pagerduty.ListExtensionsResult{
			PaginationResult: pagerduty.PaginationResult{
				More:  false,
				Total: uint(len(extensions)),
			},
			Extensions: extensions,
		}
		err := json.NewEncoder(rw).Encode(resp)
		panicIf(err)
	})
	router.POST("/extensions", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)

		var body pagerduty.ExtensionBodyWrap
		err := json.NewDecoder(r.Body).Decode(&body)
		panicIf(err)

		extension := mock.StoreExtension(pagerduty.Extension{
			Name:             body.Extension.Name,
			EndpointURL:      body.Extension.EndpointURL,
			ExtensionObjects: body.Extension.ExtensionObjects,
			ExtensionSchema:  body.Extension.ExtensionSchema,
		})
		mock.newExtensions <- extension

		err = json.NewEncoder(rw).Encode(pagerduty.ExtensionResult{Extension: extension})
		panicIf(err)
	})
	router.PUT("/extensions/:id", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		extension, found := mock.GetExtension(ps.ByName("id"))
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(&pagerduty.ErrorResult{Message: "Extension not found"})
			panicIf(err)
			return
		}

		err := json.NewDecoder(r.Body).Decode(&extension)
		panicIf(err)

		mock.StoreExtension(extension)
		mock.newExtensions <- extension

		err = json.NewEncoder(rw).Encode(pagerduty.ExtensionResult{Extension: extension})
		panicIf(err)
	})
	router.GET("/users", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		var users []pagerduty.User
		mock.objects.Range(func(key, value interface{}) bool {
			if key, ok := key.(string); !ok || !strings.HasPrefix(key, "user-") {
				return true
			}
			if user, ok := value.(pagerduty.User); ok {
				users = append(users, user)
			}
			return true
		})
		err := json.NewEncoder(rw).Encode(pagerduty.ListUsersResult{Users: users})
		panicIf(err)
	})
	router.GET("/users/:id", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		id := ps.ByName("id")
		user, found := mock.GetUser(id)
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(&pagerduty.ErrorResult{Message: "User not found"})
			panicIf(err)
			return
		}

		err := json.NewEncoder(rw).Encode(pagerduty.UserResult{User: user})
		panicIf(err)
	})
	router.GET("/incidents", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		query := QueryValues(r.URL.Query())
		serviceIDSet := query.GetAsSet("service_ids[]")
		userIDSet := query.GetAsSet("user_ids[]")

		var incidents []pagerduty.Incident

		mock.objects.Range(func(key, value interface{}) bool {
			if key, ok := key.(string); !ok || !strings.HasPrefix(key, "incident-") {
				return true
			}
			incident, ok := value.(pagerduty.Incident)
			if !ok {
				return true
			}

			// Filter by service_ids
			if serviceIDSet.Len() > 0 && (incident.Service.Type != "service_reference" || !serviceIDSet.Contains(incident.Service.ID)) {
				return true
			}

			// Filter by user_ids
			if userIDSet.Len() > 0 {
				ok := false
				for _, assignment := range incident.Assignments {
					if assignment.Assignee.Type == "user_reference" && userIDSet.Contains(assignment.Assignee.ID) {
						ok = true
						break
					}
				}
				if !ok {
					return true
				}
			}

			incidents = append(incidents, incident)

			return true
		})

		err := json.NewEncoder(rw).Encode(pagerduty.ListIncidentsResult{Incidents: incidents})
		panicIf(err)
	})
	router.POST("/incidents", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)

		var body pagerduty.IncidentBodyWrap
		err := json.NewDecoder(r.Body).Decode(&body)
		panicIf(err)

		service, found := mock.GetService(body.Incident.Service.ID)
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(&pagerduty.ErrorResult{Message: "Service not found"})
			panicIf(err)
			return
		}

		var assignments []pagerduty.IncidentAssignment
		for _, onCall := range mock.GetOnCallsByEscalationPolicy(service.EscalationPolicy.ID) {
			assignments = append(assignments, pagerduty.IncidentAssignment{Assignee: onCall.User})
		}

		incident := mock.StoreIncident(pagerduty.Incident{
			IncidentKey: body.Incident.IncidentKey,
			Title:       body.Incident.Title,
			Status:      "triggered",
			Service:     body.Incident.Service,
			Assignments: assignments,
			Body:        body.Incident.Body,
		})
		mock.newIncidents <- incident

		err = json.NewEncoder(rw).Encode(pagerduty.IncidentResult{Incident: incident})
		panicIf(err)
	})
	router.PUT("/incidents/:id", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		incident, found := mock.GetIncident(ps.ByName("id"))
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(&pagerduty.ErrorResult{Message: "Incident not found"})
			panicIf(err)
			return
		}

		var body pagerduty.IncidentBodyWrap
		err := json.NewDecoder(r.Body).Decode(&body)
		panicIf(err)

		incident.Status = body.Incident.Status
		mock.StoreIncident(incident)
		mock.incidentUpdates <- incident

		err = json.NewEncoder(rw).Encode(pagerduty.IncidentResult{Incident: incident})
		panicIf(err)
	})
	router.POST("/incidents/:id/notes", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)

		incidentID := ps.ByName("id")

		var body pagerduty.IncidentNoteBodyWrap
		err := json.NewDecoder(r.Body).Decode(&body)
		panicIf(err)

		note := mock.StoreIncidentNote(pagerduty.IncidentNote{Content: body.Note.Content})
		mock.newIncidentNotes <- FakeIncidentNote{IncidentNote: note, IncidentID: incidentID}

		err = json.NewEncoder(rw).Encode(pagerduty.IncidentNoteResult{Note: note})
		panicIf(err)
	})
	router.GET("/oncalls", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		query := QueryValues(r.URL.Query())
		userIDSet := query.GetAsSet("user_ids[]")
		policyIDSet := query.GetAsSet("escalation_policy_ids[]")

		var onCalls []pagerduty.OnCall

		mock.objects.Range(func(key, value interface{}) bool {
			if key, ok := key.(string); !ok || !strings.HasPrefix(key, "oncall-") {
				return true
			}

			onCall, ok := value.(pagerduty.OnCall)
			if !ok {
				return true
			}
			// Filter by user_ids
			if userIDSet.Len() > 0 && (onCall.User.Type != "user_reference" || !userIDSet.Contains(onCall.User.ID)) {
				return true
			}

			// Filter by escalation_policy_ids
			if policyIDSet.Len() > 0 && (onCall.EscalationPolicy.Type != "escalation_policy_reference" || !policyIDSet.Contains(onCall.EscalationPolicy.ID)) {
				return true
			}

			onCalls = append(onCalls, onCall)
			return true
		})

		err := json.NewEncoder(rw).Encode(pagerduty.ListOnCallsResult{OnCalls: onCalls})
		panicIf(err)
	})

	return mock
}

func (s *FakePagerduty) URL() string {
	return s.srv.URL
}

func (s *FakePagerduty) Close() {
	s.srv.Close()
	close(s.newExtensions)
	close(s.newIncidents)
	close(s.incidentUpdates)
	close(s.newIncidentNotes)
}

func (s *FakePagerduty) GetService(id string) (pagerduty.Service, bool) {
	if obj, ok := s.objects.Load(id); ok {
		service, ok := obj.(pagerduty.Service)
		return service, ok
	}
	return pagerduty.Service{}, false
}

func (s *FakePagerduty) GetServiceByName(name string) (pagerduty.Service, bool) {
	if obj, ok := s.objects.Load(fakeServiceByNameKey(strings.ToLower(name))); ok {
		service, ok := obj.(pagerduty.Service)
		return service, ok
	}
	return pagerduty.Service{}, false
}

func (s *FakePagerduty) StoreService(service pagerduty.Service) pagerduty.Service {
	byNameKey := fakeServiceByNameKey(strings.ToLower(service.Name))
	if service.ID == "" {
		if obj, ok := s.objects.Load(byNameKey); ok {
			service.ID = obj.(pagerduty.Service).ID
		} else {
			service.ID = fmt.Sprintf("service-%v", atomic.AddUint64(&s.serviceIDCounter, 1))
		}
	}
	s.objects.Store(service.ID, service)
	s.objects.Store(byNameKey, service)
	return service
}

func (s *FakePagerduty) GetExtension(id string) (pagerduty.Extension, bool) {
	if obj, ok := s.objects.Load(id); ok {
		extension, ok := obj.(pagerduty.Extension)
		return extension, ok
	}
	return pagerduty.Extension{}, false
}

func (s *FakePagerduty) StoreExtension(extension pagerduty.Extension) pagerduty.Extension {
	if extension.ID == "" {
		extension.ID = fmt.Sprintf("extension-%v", atomic.AddUint64(&s.extensionIDCounter, 1))
	}
	s.objects.Store(extension.ID, extension)
	return extension
}

func (s *FakePagerduty) GetUser(id string) (pagerduty.User, bool) {
	if obj, ok := s.objects.Load(id); ok {
		user, ok := obj.(pagerduty.User)
		return user, ok
	}
	return pagerduty.User{}, false
}

func (s *FakePagerduty) GetUserByEmail(email string) (pagerduty.User, bool) {
	if obj, ok := s.objects.Load(fakeUserByEmailKey(email)); ok {
		user, ok := obj.(pagerduty.User)
		return user, ok
	}
	return pagerduty.User{}, false
}

func (s *FakePagerduty) StoreUser(user pagerduty.User) pagerduty.User {
	if user.ID == "" {
		user.ID = fmt.Sprintf("user-%v", atomic.AddUint64(&s.userIDCounter, 1))
	}
	s.objects.Store(user.ID, user)
	return user
}

func (s *FakePagerduty) GetIncident(id string) (pagerduty.Incident, bool) {
	if obj, ok := s.objects.Load(id); ok {
		incident, ok := obj.(pagerduty.Incident)
		return incident, ok
	}
	return pagerduty.Incident{}, false
}

func (s *FakePagerduty) StoreIncident(incident pagerduty.Incident) pagerduty.Incident {
	if incident.ID == "" {
		incident.ID = fmt.Sprintf("incident-%v", atomic.AddUint64(&s.incidentIDCounter, 1))
	}
	s.objects.Store(incident.ID, incident)
	return incident
}

func (s *FakePagerduty) StoreIncidentNote(note pagerduty.IncidentNote) pagerduty.IncidentNote {
	if note.ID == "" {
		note.ID = fmt.Sprintf("incident_note-%v", atomic.AddUint64(&s.incidentNoteIDCounter, 1))
	}
	s.objects.Store(note.ID, note)
	return note
}

func (s *FakePagerduty) StoreOnCall(onCall pagerduty.OnCall) pagerduty.OnCall {
	id := fmt.Sprintf("oncall-%v", atomic.AddUint64(&s.onCallIDCounter, 1))
	s.objects.Store(id, onCall)
	return onCall
}

func (s *FakePagerduty) GetOnCallsByEscalationPolicy(policyID string) []pagerduty.OnCall {
	var result []pagerduty.OnCall
	s.objects.Range(func(key, value interface{}) bool {
		if key, ok := key.(string); !ok || !strings.HasPrefix(key, "oncall-") {
			return true
		}
		onCall, ok := value.(pagerduty.OnCall)
		if !ok {
			return true
		}
		if onCall.EscalationPolicy.ID == policyID {
			result = append(result, onCall)
		}
		return true
	})
	return result
}

func (s *FakePagerduty) CheckNewExtension(ctx context.Context) (pagerduty.Extension, error) {
	select {
	case extension := <-s.newExtensions:
		return extension, nil
	case <-ctx.Done():
		return pagerduty.Extension{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakePagerduty) CheckNewIncident(ctx context.Context) (pagerduty.Incident, error) {
	select {
	case incident := <-s.newIncidents:
		return incident, nil
	case <-ctx.Done():
		return pagerduty.Incident{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakePagerduty) CheckIncidentUpdate(ctx context.Context) (pagerduty.Incident, error) {
	select {
	case incident := <-s.incidentUpdates:
		return incident, nil
	case <-ctx.Done():
		return pagerduty.Incident{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakePagerduty) CheckNewIncidentNote(ctx context.Context) (FakeIncidentNote, error) {
	select {
	case note := <-s.newIncidentNotes:
		return note, nil
	case <-ctx.Done():
		return FakeIncidentNote{}, trace.Wrap(ctx.Err())
	}
}

func panicIf(err error) {
	if err != nil {
		panic(fmt.Sprintf("%v at %v", err, string(debug.Stack())))
	}
}
