/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/integrations/access/datadog"
)

type FakeDatadog struct {
	srv *httptest.Server

	newIncidents     chan datadog.IncidentsBody
	incidentUpdates  chan datadog.IncidentsBody
	newIncidentNotes chan datadog.TimelineBody

	objects               sync.Map
	incidentIDCounter     uint64
	incidentNoteIDCounter uint64
	userIDCounter         uint64
}

func NewFakeDatadog(concurrency int) *FakeDatadog {
	router := httprouter.New()
	mock := &FakeDatadog{
		srv: httptest.NewServer(router),

		newIncidents:     make(chan datadog.IncidentsBody, concurrency),
		incidentUpdates:  make(chan datadog.IncidentsBody, concurrency),
		newIncidentNotes: make(chan datadog.TimelineBody, concurrency*3),
	}

	// Ignore api version for tests
	const apiPrefix = "/" + datadog.APIVersion
	const unstablePrefix = "/" + datadog.APIUnstable
	router.NotFound = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, apiPrefix) {
			http.StripPrefix(apiPrefix, router).ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, unstablePrefix) {
			http.StripPrefix(unstablePrefix, router).ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})

	router.GET("/permissions", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		err := json.NewEncoder(rw).Encode(datadog.PermissionsBody{
			Data: []datadog.PermissionsData{
				{
					Attributes: datadog.PermissionsAttributes{
						Name:       datadog.IncidentWritePermissions,
						Restricted: false,
					},
				},
			},
		})
		panicIf(err)
	})

	router.POST("/incidents", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)

		var body datadog.IncidentsBody
		err := json.NewDecoder(r.Body).Decode(&body)
		panicIf(err)

		incident := mock.StoreIncident(body)
		mock.newIncidents <- incident

		err = json.NewEncoder(rw).Encode(incident)
		panicIf(err)
	})

	router.PATCH("/incidents/:incident_id", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		incident, found := mock.GetIncident(ps.ByName("incident_id"))
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(&datadog.ErrorResult{Errors: []string{"Incident not found"}})
			panicIf(err)
			return
		}

		var body datadog.IncidentsBody
		err := json.NewDecoder(r.Body).Decode(&body)
		panicIf(err)

		incident.Data.Attributes.Fields.State = body.Data.Attributes.Fields.State
		mock.StoreIncident(incident)
		mock.incidentUpdates <- incident

		err = json.NewEncoder(rw).Encode(incident)
		panicIf(err)
	})

	router.POST("/incidents/:incident_id/timeline", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		rw.WriteHeader(http.StatusCreated)

		var body datadog.TimelineBody
		err := json.NewDecoder(r.Body).Decode(&body)
		panicIf(err)

		note := mock.StoreIncidentNote(body)
		mock.newIncidentNotes <- note

		err = json.NewEncoder(rw).Encode(note)
		panicIf(err)
	})

	router.GET("/on-call/teams", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		oncallTeams, found := mock.GetOncallTeams()
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(&datadog.ErrorResult{Errors: []string{"On-call Teams not found"}})
			panicIf(err)
			return
		}

		body := datadog.OncallTeamsBody{
			Data:     []datadog.OncallTeamsData{},
			Included: []datadog.OncallTeamsIncluded{},
		}

		for team, users := range oncallTeams {
			oncallUsers := make([]datadog.OncallUsersData, 0, len(users))

			for _, user := range users {
				userID := strconv.FormatUint(atomic.AddUint64(&mock.userIDCounter, 1), 10)
				oncallUsers = append(oncallUsers, datadog.OncallUsersData{
					Metadata: datadog.Metadata{
						ID: userID,
					},
				})
				body.Included = append(body.Included, datadog.OncallTeamsIncluded{
					Metadata: datadog.Metadata{
						ID: userID,
					},
					Attributes: datadog.OncallTeamsIncludedAttributes{
						Email: user,
					},
				})
			}

			body.Data = append(body.Data, datadog.OncallTeamsData{
				Attributes: datadog.OncallTeamsAttributes{
					Handle: team,
				},
				Relationships: datadog.OncallTeamsRelationships{
					OncallUsers: datadog.OncallUsers{
						Data: oncallUsers,
					},
				},
			})
		}

		err := json.NewEncoder(rw).Encode(body)
		panicIf(err)
	})

	return mock
}

func (d *FakeDatadog) URL() string {
	return d.srv.URL
}

func (d *FakeDatadog) Close() {
	d.srv.Close()
	close(d.newIncidents)
	close(d.incidentUpdates)
	close(d.newIncidentNotes)
}

func (d *FakeDatadog) GetIncident(id string) (datadog.IncidentsBody, bool) {
	if obj, ok := d.objects.Load(id); ok {
		incident, ok := obj.(datadog.IncidentsBody)
		return incident, ok
	}
	return datadog.IncidentsBody{}, false
}

func (d *FakeDatadog) StoreIncident(incident datadog.IncidentsBody) datadog.IncidentsBody {
	if incident.Data.ID == "" {
		incident.Data.ID = fmt.Sprintf("incident-%v", atomic.AddUint64(&d.incidentIDCounter, 1))
	}
	d.objects.Store(incident.Data.ID, incident)
	return incident
}

func (d *FakeDatadog) StoreIncidentNote(note datadog.TimelineBody) datadog.TimelineBody {
	if note.Data.ID == "" {
		note.Data.ID = fmt.Sprintf("incident_note-%v", atomic.AddUint64(&d.incidentNoteIDCounter, 1))
	}
	d.objects.Store(note.Data.ID, note)
	return note
}

func (d *FakeDatadog) CheckNewIncident(ctx context.Context) (datadog.IncidentsBody, error) {
	select {
	case incident := <-d.newIncidents:
		return incident, nil
	case <-ctx.Done():
		return datadog.IncidentsBody{}, trace.Wrap(ctx.Err())
	}
}

func (d *FakeDatadog) CheckIncidentUpdate(ctx context.Context) (datadog.IncidentsBody, error) {
	select {
	case incident := <-d.incidentUpdates:
		return incident, nil
	case <-ctx.Done():
		return datadog.IncidentsBody{}, trace.Wrap(ctx.Err())
	}
}

func (d *FakeDatadog) CheckNewIncidentNote(ctx context.Context) (datadog.TimelineBody, error) {
	select {
	case note := <-d.newIncidentNotes:
		return note, nil
	case <-ctx.Done():
		return datadog.TimelineBody{}, trace.Wrap(ctx.Err())
	}
}

func (d *FakeDatadog) StoreOncallTeams(teamName string, users []string) map[string][]string {
	oncallTeams, ok := d.GetOncallTeams()
	if !ok {
		oncallTeams = make(map[string][]string)
	}
	oncallTeams[teamName] = users

	d.objects.Store("on-call-teams", oncallTeams)
	return oncallTeams
}

func (d *FakeDatadog) GetOncallTeams() (map[string][]string, bool) {
	if obj, ok := d.objects.Load("on-call-teams"); ok {
		oncallTeams, ok := obj.(map[string][]string)
		return oncallTeams, ok
	}
	return nil, false
}

func panicIf(err error) {
	if err != nil {
		panic(fmt.Sprintf("%v at %v", err, string(debug.Stack())))
	}
}
