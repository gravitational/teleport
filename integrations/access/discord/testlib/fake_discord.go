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
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/integrations/access/discord"
)

type FakeDiscord struct {
	srv *httptest.Server

	objects                    sync.Map
	newMessages                chan discord.DiscordMsg
	messageUpdatesByAPI        chan discord.DiscordMsg
	messageUpdatesByResponding chan discord.DiscordMsg
	messageCounter             uint64
	startTime                  time.Time
}

func NewFakeDiscord(concurrency int) *FakeDiscord {
	router := httprouter.New()

	s := &FakeDiscord{
		newMessages:                make(chan discord.DiscordMsg, concurrency*6),
		messageUpdatesByAPI:        make(chan discord.DiscordMsg, concurrency*2),
		messageUpdatesByResponding: make(chan discord.DiscordMsg, concurrency),
		startTime:                  time.Now(),
		srv:                        httptest.NewServer(router),
	}

	router.GET("/users/@me", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		err := json.NewEncoder(rw).Encode(discord.DiscordResponse{Code: http.StatusOK})
		panicIf(err)
	})

	router.POST("/channels/:channelID/messages", func(rw http.ResponseWriter, r *http.Request, params httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		var payload discord.DiscordMsg
		err := json.NewDecoder(r.Body).Decode(&payload)
		panicIf(err)

		channel := params.ByName("channelID")

		msg := s.StoreMessage(discord.DiscordMsg{Msg: discord.Msg{
			Channel: channel,
		},
			Text: payload.Text,
		})

		s.newMessages <- msg

		response := discord.ChatMsgResponse{
			DiscordResponse: discord.DiscordResponse{Code: http.StatusOK},
			Channel:         channel,
			Text:            payload.Text,
			DiscordID:       msg.DiscordID,
		}
		err = json.NewEncoder(rw).Encode(response)
		panicIf(err)
	})

	router.PATCH("/channels/:channelID/messages/:messageID", func(rw http.ResponseWriter, r *http.Request, params httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		var payload discord.DiscordMsg
		err := json.NewDecoder(r.Body).Decode(&payload)
		panicIf(err)

		channel := params.ByName("channelID")
		messageID := params.ByName("messageID")

		_, found := s.GetMessage(messageID)
		if !found {
			err := json.NewEncoder(rw).Encode(discord.DiscordResponse{Code: 10008, Message: "Unknown Message"})
			panicIf(err)
			return
		}

		msg := s.StoreMessage(discord.DiscordMsg{Msg: discord.Msg{
			Channel:   channel,
			DiscordID: messageID,
		},
			Text:   payload.Text,
			Embeds: payload.Embeds,
		})

		s.messageUpdatesByAPI <- msg

		response := discord.ChatMsgResponse{
			DiscordResponse: discord.DiscordResponse{Code: http.StatusOK},
			Channel:         channel,
			Text:            payload.Text,
			DiscordID:       msg.DiscordID,
		}
		err = json.NewEncoder(rw).Encode(response)
		panicIf(err)
	})

	return s
}

func (s *FakeDiscord) URL() string {
	return s.srv.URL
}

func (s *FakeDiscord) Close() {
	s.srv.Close()
	close(s.newMessages)
	close(s.messageUpdatesByAPI)
	close(s.messageUpdatesByResponding)
}

func (s *FakeDiscord) StoreMessage(msg discord.DiscordMsg) discord.DiscordMsg {
	if msg.DiscordID == "" {
		msg.DiscordID = strconv.FormatUint(atomic.AddUint64(&s.messageCounter, 1), 10)
	}
	s.objects.Store(fmt.Sprintf("msg-%s", msg.DiscordID), msg)
	return msg
}

func (s *FakeDiscord) GetMessage(id string) (discord.DiscordMsg, bool) {
	if obj, ok := s.objects.Load(fmt.Sprintf("msg-%s", id)); ok {
		msg, ok := obj.(discord.DiscordMsg)
		return msg, ok
	}
	return discord.DiscordMsg{}, false
}

func (s *FakeDiscord) CheckNewMessage(ctx context.Context) (discord.DiscordMsg, error) {
	select {
	case message := <-s.newMessages:
		return message, nil
	case <-ctx.Done():
		return discord.DiscordMsg{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakeDiscord) CheckMessageUpdateByAPI(ctx context.Context) (discord.DiscordMsg, error) {
	select {
	case message := <-s.messageUpdatesByAPI:
		return message, nil
	case <-ctx.Done():
		return discord.DiscordMsg{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakeDiscord) CheckMessageUpdateByResponding(ctx context.Context) (discord.DiscordMsg, error) {
	select {
	case message := <-s.messageUpdatesByResponding:
		return message, nil
	case <-ctx.Done():
		return discord.DiscordMsg{}, trace.Wrap(ctx.Err())
	}
}

func panicIf(err error) {
	if err != nil {
		panic(fmt.Sprintf("%v at %v", err, string(debug.Stack())))
	}
}
