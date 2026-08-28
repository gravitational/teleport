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
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/integrations/access/slack"
)

type FakeSlack struct {
	srv *httptest.Server

	botUser                    slack.User
	objects                    sync.Map
	newMessages                chan slack.Message
	messageUpdatesByAPI        chan slack.Message
	messageUpdatesByResponding chan slack.Message
	messageCounter             uint64
	userIDCounter              uint64
	startTime                  time.Time
}

func NewFakeSlack(botUser slack.User, concurrency int) *FakeSlack {
	router := httprouter.New()

	s := &FakeSlack{
		newMessages:                make(chan slack.Message, concurrency*6),
		messageUpdatesByAPI:        make(chan slack.Message, concurrency*2),
		messageUpdatesByResponding: make(chan slack.Message, concurrency),
		startTime:                  time.Now(),
		srv:                        httptest.NewServer(router),
	}

	s.botUser = s.StoreUser(botUser)

	router.POST("/auth.test", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		err := json.NewEncoder(rw).Encode(slack.APIResponse{Ok: true})
		panicIf(err)
	})

	router.POST("/chat.postMessage", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		var payload slack.Message
		err := json.NewDecoder(r.Body).Decode(&payload)
		panicIf(err)

		// text limit and block text limit as per
		// https://api.slack.com/methods/chat.postMessage and
		// https://api.slack.com/reference/block-kit/blocks#section
		if len(payload.Text) > 4000 || func() bool {
			for _, block := range payload.BlockItems {
				sectionBlock, ok := block.Block.(slack.SectionBlock)
				if !ok {
					continue
				}
				if len(sectionBlock.Text.GetText()) > 3000 {
					return true
				}
			}
			return false
		}() {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		msg := s.StoreMessage(slack.Message{BaseMessage: slack.BaseMessage{
			Type:     "message",
			Channel:  payload.Channel,
			ThreadTs: payload.ThreadTs,
			User:     s.botUser.ID,
			Username: s.botUser.Name,
		},
			BlockItems: payload.BlockItems,
			Text:       payload.Text,
		})
		s.newMessages <- msg

		response := slack.ChatMsgResponse{
			APIResponse: slack.APIResponse{Ok: true},
			Channel:     msg.Channel,
			Timestamp:   msg.Timestamp,
			Text:        msg.Text,
		}
		err = json.NewEncoder(rw).Encode(response)
		panicIf(err)
	})

	router.POST("/chat.update", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		var payload slack.Message
		err := json.NewDecoder(r.Body).Decode(&payload)
		panicIf(err)

		msg, found := s.GetMessage(payload.Timestamp)
		if !found {
			err := json.NewEncoder(rw).Encode(slack.APIResponse{Ok: false, Error: "message_not_found"})
			panicIf(err)
			return
		}

		msg.Text = payload.Text
		msg.BlockItems = payload.BlockItems

		s.messageUpdatesByAPI <- s.StoreMessage(msg)

		response := slack.ChatMsgResponse{
			APIResponse: slack.APIResponse{Ok: true},
			Channel:     msg.Channel,
			Timestamp:   msg.Timestamp,
			Text:        msg.Text,
		}
		err = json.NewEncoder(rw).Encode(&response)
		panicIf(err)
	})

	router.POST("/_response/:ts", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		var payload struct {
			slack.Message
			ReplaceOriginal bool `json:"replace_original"`
		}
		err := json.NewDecoder(r.Body).Decode(&payload)
		panicIf(err)

		timestamp := ps.ByName("ts")
		msg, found := s.GetMessage(timestamp)
		if !found {
			err := json.NewEncoder(rw).Encode(slack.APIResponse{Ok: false, Error: "message_not_found"})
			panicIf(err)
			return
		}

		if payload.ReplaceOriginal {
			msg.BlockItems = payload.BlockItems
			s.messageUpdatesByResponding <- s.StoreMessage(msg)
		} else {
			newMsg := s.StoreMessage(slack.Message{BaseMessage: slack.BaseMessage{
				Type:     "message",
				Channel:  msg.Channel,
				User:     s.botUser.ID,
				Username: s.botUser.Name,
			},
				BlockItems: payload.BlockItems,
			})
			s.newMessages <- newMsg
		}
		err = json.NewEncoder(rw).Encode(slack.APIResponse{Ok: true})
		panicIf(err)
	})

	router.GET("/users.info", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		id := r.URL.Query().Get("user")
		if id == "" {
			err := json.NewEncoder(rw).Encode(slack.APIResponse{Ok: false, Error: "invalid_arguments"})
			panicIf(err)
			return
		}

		user, found := s.GetUser(id)
		if !found {
			err := json.NewEncoder(rw).Encode(slack.APIResponse{Ok: false, Error: "user_not_found"})
			panicIf(err)
			return
		}

		err := json.NewEncoder(rw).Encode(struct {
			User slack.User `json:"user"`
			Ok   bool       `json:"ok"`
		}{user, true})
		panicIf(err)
	})

	router.GET("/users.lookupByEmail", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		email := r.URL.Query().Get("email")
		if email == "" {
			err := json.NewEncoder(rw).Encode(slack.APIResponse{Ok: false, Error: "invalid_arguments"})
			panicIf(err)
			return
		}

		user, found := s.GetUserByEmail(email)
		if !found {
			err := json.NewEncoder(rw).Encode(slack.APIResponse{Ok: false, Error: "users_not_found"})
			panicIf(err)
			return
		}

		err := json.NewEncoder(rw).Encode(struct {
			User slack.User `json:"user"`
			Ok   bool       `json:"ok"`
		}{user, true})
		panicIf(err)
	})

	return s
}

func (s *FakeSlack) URL() string {
	return s.srv.URL
}

func (s *FakeSlack) Close() {
	s.srv.Close()
	close(s.newMessages)
	close(s.messageUpdatesByAPI)
	close(s.messageUpdatesByResponding)
}

func (s *FakeSlack) StoreMessage(msg slack.Message) slack.Message {
	if msg.Timestamp == "" {
		now := s.startTime.Add(time.Since(s.startTime)) // get monotonic timestamp
		uniq := atomic.AddUint64(&s.messageCounter, 1)  // generate uniq int to prevent races
		msg.Timestamp = fmt.Sprintf("%d.%d", now.UnixNano(), uniq)
	}
	s.objects.Store(fmt.Sprintf("msg-%s", msg.Timestamp), msg)
	return msg
}

func (s *FakeSlack) GetMessage(id string) (slack.Message, bool) {
	if obj, ok := s.objects.Load(fmt.Sprintf("msg-%s", id)); ok {
		msg, ok := obj.(slack.Message)
		return msg, ok
	}
	return slack.Message{}, false
}

func (s *FakeSlack) StoreUser(user slack.User) slack.User {
	if user.ID == "" {
		user.ID = fmt.Sprintf("U%d", atomic.AddUint64(&s.userIDCounter, 1))
	}
	s.objects.Store(fmt.Sprintf("user-%s", user.ID), user)
	s.objects.Store(fmt.Sprintf("userByEmail-%s", user.Profile.Email), user)
	return user
}

func (s *FakeSlack) GetUser(id string) (slack.User, bool) {
	if obj, ok := s.objects.Load(fmt.Sprintf("user-%s", id)); ok {
		user, ok := obj.(slack.User)
		return user, ok
	}
	return slack.User{}, false
}

func (s *FakeSlack) GetUserByEmail(email string) (slack.User, bool) {
	if obj, ok := s.objects.Load(fmt.Sprintf("userByEmail-%s", email)); ok {
		user, ok := obj.(slack.User)
		return user, ok
	}
	return slack.User{}, false
}

func (s *FakeSlack) CheckNewMessage(ctx context.Context) (slack.Message, error) {
	select {
	case message := <-s.newMessages:
		return message, nil
	case <-ctx.Done():
		return slack.Message{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakeSlack) CheckMessageUpdateByAPI(ctx context.Context) (slack.Message, error) {
	select {
	case message := <-s.messageUpdatesByAPI:
		return message, nil
	case <-ctx.Done():
		return slack.Message{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakeSlack) CheckMessageUpdateByResponding(ctx context.Context) (slack.Message, error) {
	select {
	case message := <-s.messageUpdatesByResponding:
		return message, nil
	case <-ctx.Done():
		return slack.Message{}, trace.Wrap(ctx.Err())
	}
}

func panicIf(err error) {
	if err != nil {
		panic(fmt.Sprintf("%v at %v", err, string(debug.Stack())))
	}
}
