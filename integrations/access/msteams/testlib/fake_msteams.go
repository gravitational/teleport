// Copyright 2024 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testlib

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/integrations/access/msteams/msapi"
)

type response struct {
	Value any `json:"value"`
}

type Msg struct {
	ID          string
	RecipientID string
	Body        string
}

type FakeTeams struct {
	srv *httptest.Server

	msapi.Config

	teamsApp msapi.TeamsApp

	objects         sync.Map
	newMessages     chan Msg
	updatedMessages chan Msg
	userIDCounter   uint64
	startTime       time.Time
}

func NewFakeTeams(concurrency int) *FakeTeams {
	router := httprouter.New()

	s := &FakeTeams{
		newMessages:     make(chan Msg, concurrency*2),
		updatedMessages: make(chan Msg, concurrency*2),
		startTime:       time.Now(),
		srv:             httptest.NewServer(router),
		Config: msapi.Config{
			AppID:      uuid.NewString(),
			AppSecret:  uuid.NewString(),
			TenantID:   uuid.NewString(),
			TeamsAppID: uuid.NewString(),
		},
	}

	s.teamsApp = msapi.TeamsApp{
		ID:          s.Config.AppID,
		ExternalID:  s.Config.TeamsAppID,
		DisplayName: "Teleport Bot",
	}

	router.POST("/"+s.Config.TenantID+"/oauth2/v2.0/token", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		err := json.NewEncoder(rw).Encode(response{Value: msapi.Token{
			AccessToken: uuid.New().String(),
			ExpiresIn:   3600,
		}})
		panicIf(err)
	})

	router.GET("/appCatalogs/teamsApps", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		err := json.NewEncoder(rw).Encode(response{Value: []msapi.TeamsApp{s.teamsApp}})
		panicIf(err)
	})

	router.GET("/users", func(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
		var v any = []msapi.User{}

		filter := r.URL.Query().Get("$filter")
		value := filter[strings.Index(filter, "'")+1 : len(filter)-1]

		if strings.Contains(filter, "mail eq") {
			u, ok := s.GetUserByEmail(value)
			if ok {
				v = []msapi.User{u}
			}
		}

		if strings.Contains(filter, "id eq") {
			u, ok := s.GetUser(value)
			if ok {
				v = []msapi.User{u}
			}
		}

		err := json.NewEncoder(rw).Encode(response{Value: v})
		panicIf(err)
	})

	router.POST("/users/:userID/teamWork/installedApps", func(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
		var v struct {
			URL string `json:"teamsApp@odata.bind"`
		}

		_, ok := s.GetUser(p.ByName("userID"))
		if !ok {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		err := json.NewDecoder(r.Body).Decode(&v)
		id := v.URL[strings.LastIndex(v.URL, "/")+1 : len(v.URL)]

		s.StoreApp(msapi.InstalledApp{ID: id})

		rw.WriteHeader(http.StatusCreated)
		panicIf(err)
	})

	router.GET("/users/:userID/teamWork/installedApps", func(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
		var v any = []msapi.InstalledApp{}

		_, ok := s.GetUser(p.ByName("userID"))
		if !ok {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		filter := r.URL.Query().Get("$filter")
		id := filter[strings.Index(filter, "'")+1 : len(filter)-1]

		a, ok := s.GetApp(id)
		if ok {
			v = []msapi.InstalledApp{a}
		}
		err := json.NewEncoder(rw).Encode(response{Value: v})
		panicIf(err)
	})

	router.GET("/users/:userID/teamWork/installedApps/:appID/chat", func(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
		var c msapi.Chat

		_, ok := s.GetUser(p.ByName("userID"))
		if !ok {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		_, ok = s.GetApp(p.ByName("appID"))
		if !ok {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		id := uuid.NewString()
		c = msapi.Chat{ID: id, TenantID: p.ByName("userID")}
		s.StoreChat(c)

		err := json.NewEncoder(rw).Encode(c)
		panicIf(err)
	})

	router.POST("/emea/v3/conversations/:chatID/activities", func(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		b, err := io.ReadAll(r.Body)
		panicIf(err)

		id := uuid.NewString()

		c, ok := s.GetChat(p.ByName("chatID"))
		if !ok {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		msg := Msg{
			ID:          id,
			RecipientID: c.TenantID,
			Body:        string(b),
		}

		s.newMessages <- msg

		rw.WriteHeader(http.StatusCreated)

		_, err = rw.Write([]byte(`{"id":"` + id + `"}`))
		panicIf(err)
	})

	router.PUT("/emea/v3/conversations/:chatID/activities/:id", func(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		b, err := io.ReadAll(r.Body)
		panicIf(err)

		id := p.ByName("chatID")

		c, ok := s.GetChat(id)
		if !ok {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		msg := Msg{
			ID:          p.ByName("id"),
			RecipientID: c.TenantID,
			Body:        string(b),
		}

		s.updatedMessages <- msg

		rw.WriteHeader(http.StatusOK)

		_, err = rw.Write([]byte(`{"id":"` + id + `"}`))
		panicIf(err)
	})

	return s
}

func (s *FakeTeams) URL() string {
	return s.srv.URL
}

func (s *FakeTeams) Close() {
	s.srv.Close()
	close(s.newMessages)
	close(s.updatedMessages)
}

func (s *FakeTeams) StoreUser(user msapi.User) msapi.User {
	if user.ID == "" {
		user.ID = fmt.Sprintf("U%d", atomic.AddUint64(&s.userIDCounter, 1))
	}

	s.objects.Store(fmt.Sprintf("user-%s", user.ID), user)
	s.objects.Store(fmt.Sprintf("userByEmail-%s", user.Mail), user)

	return user
}

func (s *FakeTeams) StoreApp(a msapi.InstalledApp) msapi.InstalledApp {
	a.TeamsApp = s.teamsApp
	s.objects.Store(fmt.Sprintf("app-%s", a.ID), a)
	return a
}

func (s *FakeTeams) StoreChat(c msapi.Chat) msapi.Chat {
	s.objects.Store(fmt.Sprintf("chat-%s", c.ID), c)
	return c
}

func (s *FakeTeams) GetUser(id string) (msapi.User, bool) {
	if obj, ok := s.objects.Load(fmt.Sprintf("user-%s", id)); ok {
		user, ok := obj.(msapi.User)
		return user, ok
	}
	return msapi.User{}, false
}

func (s *FakeTeams) GetUserByEmail(email string) (msapi.User, bool) {
	if obj, ok := s.objects.Load(fmt.Sprintf("userByEmail-%s", email)); ok {
		user, ok := obj.(msapi.User)
		return user, ok
	}
	return msapi.User{}, false
}

func (s *FakeTeams) GetApp(id string) (msapi.InstalledApp, bool) {
	if obj, ok := s.objects.Load(fmt.Sprintf("app-%s", id)); ok {
		user, ok := obj.(msapi.InstalledApp)
		return user, ok
	}
	return msapi.InstalledApp{}, false
}

func (s *FakeTeams) GetChat(id string) (msapi.Chat, bool) {
	if obj, ok := s.objects.Load(fmt.Sprintf("chat-%s", id)); ok {
		user, ok := obj.(msapi.Chat)
		return user, ok
	}
	return msapi.Chat{}, false
}

func (s *FakeTeams) CheckNewMessage(ctx context.Context) (Msg, error) {
	select {
	case message := <-s.newMessages:
		return message, nil
	case <-ctx.Done():
		return Msg{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakeTeams) CheckMessageUpdate(ctx context.Context) (Msg, error) {
	select {
	case message := <-s.updatedMessages:
		return message, nil
	case <-ctx.Done():
		return Msg{}, trace.Wrap(ctx.Err())
	}
}

func panicIf(err error) {
	if err != nil {
		panic(fmt.Sprintf("%v at %v", err, string(debug.Stack())))
	}
}
