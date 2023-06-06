/*
Copyright 2020-2021 Gravitational, Inc.

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

package pagerduty

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

	"github.com/gravitational/teleport/integrations/lib/stringset"
)

type FakePagerduty struct {
	srv *httptest.Server

	objects sync.Map
	// Extensions
	extensionIDCounter uint64
	newExtensions      chan Extension
	// Inicidents
	incidentIDCounter uint64
	newIncidents      chan Incident
	incidentUpdates   chan Incident
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
	IncidentNote
}

func NewFakePagerduty(concurrency int) *FakePagerduty {
	router := httprouter.New()

	pagerduty := &FakePagerduty{
		newExtensions:    make(chan Extension, concurrency),
		newIncidents:     make(chan Incident, concurrency),
		incidentUpdates:  make(chan Incident, concurrency),
		newIncidentNotes: make(chan FakeIncidentNote, concurrency*3), // for any incident there could be 1-3 notes
		srv:              httptest.NewServer(router),
	}

	router.GET("/services/:id", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		id := ps.ByName("id")
		service, found := pagerduty.GetService(id)
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(ErrorResult{Message: "Service not found"})
			panicIf(err)
			return
		}

		err := json.NewEncoder(rw).Encode(ServiceResult{Service: service})
		panicIf(err)
	})
	router.GET("/services", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		var services []Service
		if query := r.URL.Query().Get("query"); query != "" {
			if service, ok := pagerduty.GetServiceByName(query); ok {
				services = append(services, service)
			}
		} else {
			pagerduty.objects.Range(func(key, value interface{}) bool {
				if key, ok := key.(string); !ok || !strings.HasPrefix(key, "service-") {
					return true
				}
				if service, ok := value.(Service); ok {
					services = append(services, service)
				}
				return true
			})
		}

		err := json.NewEncoder(rw).Encode(ListServicesResult{Services: services})
		panicIf(err)
	})
	router.GET("/extension_schemas", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		resp := ListExtensionSchemasResult{
			PaginationResult: PaginationResult{
				More:  false,
				Total: 1,
			},
			ExtensionSchemas: []ExtensionSchema{
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

		extensions := []Extension{}
		pagerduty.objects.Range(func(key, value interface{}) bool {
			if extension, ok := value.(Extension); ok {
				extensions = append(extensions, extension)
			}
			return true
		})
		resp := ListExtensionsResult{
			PaginationResult: PaginationResult{
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

		var body ExtensionBodyWrap
		err := json.NewDecoder(r.Body).Decode(&body)
		panicIf(err)

		extension := pagerduty.StoreExtension(Extension{
			Name:             body.Extension.Name,
			EndpointURL:      body.Extension.EndpointURL,
			ExtensionObjects: body.Extension.ExtensionObjects,
			ExtensionSchema:  body.Extension.ExtensionSchema,
		})
		pagerduty.newExtensions <- extension

		err = json.NewEncoder(rw).Encode(ExtensionResult{Extension: extension})
		panicIf(err)
	})
	router.PUT("/extensions/:id", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		extension, found := pagerduty.GetExtension(ps.ByName("id"))
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(&ErrorResult{Message: "Extension not found"})
			panicIf(err)
			return
		}

		err := json.NewDecoder(r.Body).Decode(&extension)
		panicIf(err)

		pagerduty.StoreExtension(extension)
		pagerduty.newExtensions <- extension

		err = json.NewEncoder(rw).Encode(ExtensionResult{Extension: extension})
		panicIf(err)
	})
	router.GET("/users", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		var users []User
		pagerduty.objects.Range(func(key, value interface{}) bool {
			if key, ok := key.(string); !ok || !strings.HasPrefix(key, "user-") {
				return true
			}
			if user, ok := value.(User); ok {
				users = append(users, user)
			}
			return true
		})
		err := json.NewEncoder(rw).Encode(ListUsersResult{Users: users})
		panicIf(err)
	})
	router.GET("/users/:id", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		id := ps.ByName("id")
		user, found := pagerduty.GetUser(id)
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(&ErrorResult{Message: "User not found"})
			panicIf(err)
			return
		}

		err := json.NewEncoder(rw).Encode(UserResult{User: user})
		panicIf(err)
	})
	router.GET("/incidents", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		query := QueryValues(r.URL.Query())
		serviceIDSet := query.GetAsSet("service_ids[]")
		userIDSet := query.GetAsSet("user_ids[]")

		var incidents []Incident

		pagerduty.objects.Range(func(key, value interface{}) bool {
			if key, ok := key.(string); !ok || !strings.HasPrefix(key, "incident-") {
				return true
			}
			incident, ok := value.(Incident)
			if !ok {
				return true
			}

			// Filter by service_ids
			if serviceIDSet.Len() > 0 && !(incident.Service.Type == "service_reference" && serviceIDSet.Contains(incident.Service.ID)) {
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

		err := json.NewEncoder(rw).Encode(ListIncidentsResult{Incidents: incidents})
		panicIf(err)
	})
	router.POST("/incidents", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)

		var body IncidentBodyWrap
		err := json.NewDecoder(r.Body).Decode(&body)
		panicIf(err)

		service, found := pagerduty.GetService(body.Incident.Service.ID)
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(&ErrorResult{Message: "Service not found"})
			panicIf(err)
			return
		}

		var assignments []IncidentAssignment
		for _, onCall := range pagerduty.GetOnCallsByEscalationPolicy(service.EscalationPolicy.ID) {
			assignments = append(assignments, IncidentAssignment{Assignee: onCall.User})
		}

		incident := pagerduty.StoreIncident(Incident{
			IncidentKey: body.Incident.IncidentKey,
			Title:       body.Incident.Title,
			Status:      "triggered",
			Service:     body.Incident.Service,
			Assignments: assignments,
			Body:        body.Incident.Body,
		})
		pagerduty.newIncidents <- incident

		err = json.NewEncoder(rw).Encode(IncidentResult{Incident: incident})
		panicIf(err)
	})
	router.PUT("/incidents/:id", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		incident, found := pagerduty.GetIncident(ps.ByName("id"))
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(&ErrorResult{Message: "Incident not found"})
			panicIf(err)
			return
		}

		var body IncidentBodyWrap
		err := json.NewDecoder(r.Body).Decode(&body)
		panicIf(err)

		incident.Status = body.Incident.Status
		pagerduty.StoreIncident(incident)
		pagerduty.incidentUpdates <- incident

		err = json.NewEncoder(rw).Encode(IncidentResult{Incident: incident})
		panicIf(err)
	})
	router.POST("/incidents/:id/notes", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)

		incidentID := ps.ByName("id")

		var body IncidentNoteBodyWrap
		err := json.NewDecoder(r.Body).Decode(&body)
		panicIf(err)

		note := pagerduty.StoreIncidentNote(IncidentNote{Content: body.Note.Content})
		pagerduty.newIncidentNotes <- FakeIncidentNote{IncidentNote: note, IncidentID: incidentID}

		err = json.NewEncoder(rw).Encode(IncidentNoteResult{Note: note})
		panicIf(err)
	})
	router.GET("/oncalls", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		query := QueryValues(r.URL.Query())
		userIDSet := query.GetAsSet("user_ids[]")
		policyIDSet := query.GetAsSet("escalation_policy_ids[]")

		var onCalls []OnCall

		pagerduty.objects.Range(func(key, value interface{}) bool {
			if key, ok := key.(string); !ok || !strings.HasPrefix(key, "oncall-") {
				return true
			}

			onCall, ok := value.(OnCall)
			if !ok {
				return true
			}
			// Filter by user_ids
			if userIDSet.Len() > 0 && !(onCall.User.Type == "user_reference" && userIDSet.Contains(onCall.User.ID)) {
				return true
			}

			// Filter by escalation_policy_ids
			if policyIDSet.Len() > 0 && !(onCall.EscalationPolicy.Type == "escalation_policy_reference" && policyIDSet.Contains(onCall.EscalationPolicy.ID)) {
				return true
			}

			onCalls = append(onCalls, onCall)
			return true
		})

		err := json.NewEncoder(rw).Encode(ListOnCallsResult{OnCalls: onCalls})
		panicIf(err)
	})

	return pagerduty
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

func (s *FakePagerduty) GetService(id string) (Service, bool) {
	if obj, ok := s.objects.Load(id); ok {
		service, ok := obj.(Service)
		return service, ok
	}
	return Service{}, false
}

func (s *FakePagerduty) GetServiceByName(name string) (Service, bool) {
	if obj, ok := s.objects.Load(fakeServiceByNameKey(strings.ToLower(name))); ok {
		service, ok := obj.(Service)
		return service, ok
	}
	return Service{}, false
}

func (s *FakePagerduty) StoreService(service Service) Service {
	byNameKey := fakeServiceByNameKey(strings.ToLower(service.Name))
	if service.ID == "" {
		if obj, ok := s.objects.Load(byNameKey); ok {
			service.ID = obj.(Service).ID
		} else {
			service.ID = fmt.Sprintf("service-%v", atomic.AddUint64(&s.serviceIDCounter, 1))
		}
	}
	s.objects.Store(service.ID, service)
	s.objects.Store(byNameKey, service)
	return service
}

func (s *FakePagerduty) GetExtension(id string) (Extension, bool) {
	if obj, ok := s.objects.Load(id); ok {
		extension, ok := obj.(Extension)
		return extension, ok
	}
	return Extension{}, false
}

func (s *FakePagerduty) StoreExtension(extension Extension) Extension {
	if extension.ID == "" {
		extension.ID = fmt.Sprintf("extension-%v", atomic.AddUint64(&s.extensionIDCounter, 1))
	}
	s.objects.Store(extension.ID, extension)
	return extension
}

func (s *FakePagerduty) GetUser(id string) (User, bool) {
	if obj, ok := s.objects.Load(id); ok {
		user, ok := obj.(User)
		return user, ok
	}
	return User{}, false
}

func (s *FakePagerduty) GetUserByEmail(email string) (User, bool) {
	if obj, ok := s.objects.Load(fakeUserByEmailKey(email)); ok {
		user, ok := obj.(User)
		return user, ok
	}
	return User{}, false
}

func (s *FakePagerduty) StoreUser(user User) User {
	if user.ID == "" {
		user.ID = fmt.Sprintf("user-%v", atomic.AddUint64(&s.userIDCounter, 1))
	}
	s.objects.Store(user.ID, user)
	return user
}

func (s *FakePagerduty) GetIncident(id string) (Incident, bool) {
	if obj, ok := s.objects.Load(id); ok {
		incident, ok := obj.(Incident)
		return incident, ok
	}
	return Incident{}, false
}

func (s *FakePagerduty) StoreIncident(incident Incident) Incident {
	if incident.ID == "" {
		incident.ID = fmt.Sprintf("incident-%v", atomic.AddUint64(&s.incidentIDCounter, 1))
	}
	s.objects.Store(incident.ID, incident)
	return incident
}

func (s *FakePagerduty) StoreIncidentNote(note IncidentNote) IncidentNote {
	if note.ID == "" {
		note.ID = fmt.Sprintf("incident_note-%v", atomic.AddUint64(&s.incidentNoteIDCounter, 1))
	}
	s.objects.Store(note.ID, note)
	return note
}

func (s *FakePagerduty) StoreOnCall(onCall OnCall) OnCall {
	id := fmt.Sprintf("oncall-%v", atomic.AddUint64(&s.onCallIDCounter, 1))
	s.objects.Store(id, onCall)
	return onCall
}

func (s *FakePagerduty) GetOnCallsByEscalationPolicy(policyID string) []OnCall {
	var result []OnCall
	s.objects.Range(func(key, value interface{}) bool {
		if key, ok := key.(string); !ok || !strings.HasPrefix(key, "oncall-") {
			return true
		}
		onCall, ok := value.(OnCall)
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

func (s *FakePagerduty) CheckNewExtension(ctx context.Context) (Extension, error) {
	select {
	case extension := <-s.newExtensions:
		return extension, nil
	case <-ctx.Done():
		return Extension{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakePagerduty) CheckNewIncident(ctx context.Context) (Incident, error) {
	select {
	case incident := <-s.newIncidents:
		return incident, nil
	case <-ctx.Done():
		return Incident{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakePagerduty) CheckIncidentUpdate(ctx context.Context) (Incident, error) {
	select {
	case incident := <-s.incidentUpdates:
		return incident, nil
	case <-ctx.Done():
		return Incident{}, trace.Wrap(ctx.Err())
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
		log.Panicf("%v at %v", err, string(debug.Stack()))
	}
}
