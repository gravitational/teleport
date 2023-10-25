// Copyright 2023 Gravitational, Inc
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

package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/require"
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

type chatResponseFull struct {
	Channel            string `json:"channel"`
	Timestamp          string `json:"ts"`                             // Regular message timestamp
	MessageTimeStamp   string `json:"message_ts"`                     // Ephemeral message timestamp
	ScheduledMessageID string `json:"scheduled_message_id,omitempty"` // Scheduled message id
	Text               string `json:"text"`
	slack.SlackResponse
}

func NewFakeSlack(t *testing.T, botUser slack.User, concurrency int) *FakeSlack {
	router := httprouter.New()

	s := &FakeSlack{
		newMessages:                make(chan slack.Message, concurrency*6),
		messageUpdatesByAPI:        make(chan slack.Message, concurrency*2),
		messageUpdatesByResponding: make(chan slack.Message, concurrency),
		startTime:                  time.Now(),
		srv:                        httptest.NewServer(router),
	}

	s.botUser = s.StoreUser(botUser)

	formToMessage := func(values url.Values) slack.Message {
		replaceOriginal := false
		if field := values.Get("replace_original"); field != "" {
			var err error
			replaceOriginal, err = strconv.ParseBool(field)
			require.NoError(t, err)
		}
		payload := slack.Message{
			Msg: slack.Msg{
				Channel:         values.Get("channel"),
				Timestamp:       values.Get("ts"),
				ThreadTimestamp: values.Get("thread_ts"),
				Text:            values.Get("text"),
				ReplaceOriginal: replaceOriginal,
			},
		}

		blocks := values.Get("blocks")
		if blocks != "" {
			json.Unmarshal([]byte(blocks), &payload.Blocks)
		}

		return payload
	}

	router.POST("/auth.test", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(rw).Encode(slack.SlackResponse{Ok: true}))
	})

	router.POST("/chat.postMessage", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		require.NoError(t, r.ParseForm())
		payload := formToMessage(r.PostForm)

		// text limit and block text limit as per
		// https://api.slack.com/methods/chat.postMessage and
		// https://api.slack.com/reference/block-kit/blocks#section
		if len(payload.Text) > 4000 || func() bool {
			for _, block := range payload.Blocks.BlockSet {
				sectionBlock, ok := block.(*slack.SectionBlock)
				if !ok {
					continue
				}
				if len(sectionBlock.Text.Text) > 3000 {
					return true
				}
			}
			return false
		}() {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		msg := s.StoreMessage(slack.Message{Msg: slack.Msg{
			Type:            "message",
			Channel:         payload.Channel,
			ThreadTimestamp: payload.ThreadTimestamp,
			User:            s.botUser.ID,
			Username:        s.botUser.Name,
			Blocks:          payload.Blocks,
			Text:            payload.Text,
		}})
		s.newMessages <- msg

		response := chatResponseFull{
			SlackResponse: slack.SlackResponse{Ok: true},
			Channel:       msg.Channel,
			Timestamp:     msg.Timestamp,
			Text:          msg.Text,
		}
		require.NoError(t, json.NewEncoder(rw).Encode(response))
	})

	router.POST("/chat.update", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		require.NoError(t, r.ParseForm())
		payload := formToMessage(r.PostForm)

		msg, found := s.GetMessage(payload.Timestamp)
		if !found {
			require.NoError(t, json.NewEncoder(rw).Encode(slack.SlackResponse{Ok: false, Error: "message_not_found"}))
			return
		}

		msg.Text = payload.Text
		msg.Blocks = payload.Blocks

		s.messageUpdatesByAPI <- s.StoreMessage(msg)

		response := chatResponseFull{
			SlackResponse: slack.SlackResponse{Ok: true},
			Channel:       msg.Channel,
			Timestamp:     msg.Timestamp,
			Text:          msg.Text,
		}
		require.NoError(t, json.NewEncoder(rw).Encode(&response))
	})

	router.POST("/_response/:ts", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		require.NoError(t, r.ParseForm())
		payload := formToMessage(r.PostForm)

		timestamp := ps.ByName("ts")
		msg, found := s.GetMessage(timestamp)
		if !found {
			require.NoError(t, json.NewEncoder(rw).Encode(slack.SlackResponse{Ok: false, Error: "message_not_found"}))
			return
		}

		if payload.ReplaceOriginal {
			msg.Blocks = payload.Blocks
			s.messageUpdatesByResponding <- s.StoreMessage(msg)
		} else {
			newMsg := s.StoreMessage(slack.Message{Msg: slack.Msg{
				Type:     "message",
				Channel:  msg.Channel,
				User:     s.botUser.ID,
				Username: s.botUser.Name,
				Blocks:   payload.Blocks,
			},
			})
			s.newMessages <- newMsg
		}
		require.NoError(t, json.NewEncoder(rw).Encode(slack.SlackResponse{Ok: true}))
	})

	router.GET("/users.info", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		id := r.URL.Query().Get("user")
		if id == "" {
			require.NoError(t, json.NewEncoder(rw).Encode(slack.SlackResponse{Ok: false, Error: "invalid_arguments"}))
			return
		}

		user, found := s.GetUser(id)
		if !found {
			require.NoError(t, json.NewEncoder(rw).Encode(slack.SlackResponse{Ok: false, Error: "user_not_found"}))
			return
		}

		require.NoError(t, json.NewEncoder(rw).Encode(struct {
			User slack.User `json:"user"`
			Ok   bool       `json:"ok"`
		}{user, true}))
	})

	router.POST("/users.lookupByEmail", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		require.NoError(t, r.ParseForm())
		email := r.PostForm.Get("email")
		if email == "" {
			require.NoError(t, json.NewEncoder(rw).Encode(slack.SlackResponse{Ok: false, Error: "invalid_arguments"}))
			return
		}

		user, found := s.GetUserByEmail(email)
		if !found {
			require.NoError(t, json.NewEncoder(rw).Encode(slack.SlackResponse{Ok: false, Error: "users_not_found"}))
			return
		}

		require.NoError(t, json.NewEncoder(rw).Encode(struct {
			User slack.User `json:"user"`
			Ok   bool       `json:"ok"`
		}{user, true}))
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
