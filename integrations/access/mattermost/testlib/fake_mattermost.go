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
	"sort"
	"sync"
	"sync/atomic"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/integrations/access/mattermost"
)

type FakeMattermost struct {
	srv         *httptest.Server
	objects     sync.Map
	botUserID   string
	newPosts    chan mattermost.Post
	postUpdates chan mattermost.Post

	postIDCounter    uint64
	userIDCounter    uint64
	teamIDCounter    uint64
	channelIDCounter uint64
}

type fakeUserByEmailKey string
type fakeTeamByNameKey string
type fakeChannelByTeamNameAndNameKey struct {
	team    string
	channel string
}
type fakeDirectChannelUsersKey struct {
	user1ID string
	user2ID string
}
type fakeDirectChannelKey string

type FakeDirectChannel struct {
	User1ID string
	User2ID string
	mattermost.Channel
}

func NewFakeMattermost(botUser mattermost.User, concurrency int) *FakeMattermost {
	router := httprouter.New()

	mock := &FakeMattermost{
		newPosts:    make(chan mattermost.Post, concurrency*6),
		postUpdates: make(chan mattermost.Post, concurrency*2),
		srv:         httptest.NewServer(router),
	}
	mock.botUserID = mock.StoreUser(botUser).ID

	router.GET("/api/v4/teams/name/:team", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		name := ps.ByName("team")
		team, found := mock.GetTeamByName(name)
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(mattermost.ErrorResult{StatusCode: http.StatusNotFound, Message: "Unable to find the team."})
			panicIf(err)
			return
		}
		err := json.NewEncoder(rw).Encode(team)
		panicIf(err)
	})

	router.GET("/api/v4/teams/name/:team/channels/name/:channel", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")
		teamName := ps.ByName("team")
		name := ps.ByName("channel")
		channel, found := mock.GetChannelByTeamNameAndName(teamName, name)
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(mattermost.ErrorResult{StatusCode: http.StatusNotFound, Message: "Unable to find the channel."})
			panicIf(err)
			return
		}
		err := json.NewEncoder(rw).Encode(channel)
		panicIf(err)
	})

	router.POST("/api/v4/channels/direct", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		var userIDs []string
		err := json.NewDecoder(r.Body).Decode(&userIDs)
		panicIf(err)
		if len(userIDs) != 2 {
			rw.WriteHeader(http.StatusBadRequest)
			err := json.NewEncoder(rw).Encode(mattermost.ErrorResult{StatusCode: http.StatusBadRequest, Message: "Expected only two user IDs."})
			panicIf(err)
			return
		}

		user1, found := mock.GetUser(userIDs[0])
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(mattermost.ErrorResult{StatusCode: http.StatusNotFound, Message: "Unable to find the user."})
			panicIf(err)
			return
		}

		user2, found := mock.GetUser(userIDs[1])
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(mattermost.ErrorResult{StatusCode: http.StatusNotFound, Message: "Unable to find the user."})
			panicIf(err)
			return
		}

		err = json.NewEncoder(rw).Encode(mock.GetDirectChannelFor(user1, user2).Channel)
		panicIf(err)
	})

	router.GET("/api/v4/users/me", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		err := json.NewEncoder(rw).Encode(mock.GetBotUser())
		panicIf(err)
	})

	router.GET("/api/v4/users/email/:email", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		email := ps.ByName("email")
		user, found := mock.GetUserByEmail(email)
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(mattermost.ErrorResult{StatusCode: http.StatusNotFound, Message: "Unable to find the user."})
			panicIf(err)
			return
		}
		err := json.NewEncoder(rw).Encode(user)
		panicIf(err)
	})

	router.POST("/api/v4/posts", func(rw http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		var post mattermost.Post
		err := json.NewDecoder(r.Body).Decode(&post)
		panicIf(err)

		// message size limit as per
		// https://github.com/mattermost/mattermost-server/blob/3d412b14af49701d842e72ef208f0ec0a35ce063/model/post.go#L54
		// (current master at time of writing)
		if len(post.Message) > 4000 {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		post = mock.StorePost(post)
		mock.newPosts <- post

		rw.WriteHeader(http.StatusCreated)
		err = json.NewEncoder(rw).Encode(post)
		panicIf(err)

	})

	router.PUT("/api/v4/posts/:id", func(rw http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		rw.Header().Add("Content-Type", "application/json")

		id := ps.ByName("id")
		post, found := mock.GetPost(id)
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			err := json.NewEncoder(rw).Encode(mattermost.ErrorResult{StatusCode: http.StatusNotFound, Message: "Unable to find the post."})
			panicIf(err)
			return
		}

		var newPost mattermost.Post
		err := json.NewDecoder(r.Body).Decode(&newPost)
		panicIf(err)

		post.Message = newPost.Message
		post.Props = newPost.Props
		post = mock.UpdatePost(post)

		rw.WriteHeader(http.StatusOK)
		err = json.NewEncoder(rw).Encode(post)
		panicIf(err)
	})

	return mock
}

func (s *FakeMattermost) URL() string {
	return s.srv.URL
}

func (s *FakeMattermost) Close() {
	s.srv.Close()
	close(s.newPosts)
	close(s.postUpdates)
}

func (s *FakeMattermost) GetPost(id string) (mattermost.Post, bool) {
	if obj, ok := s.objects.Load(id); ok {
		post, ok := obj.(mattermost.Post)
		return post, ok
	}
	return mattermost.Post{}, false
}

func (s *FakeMattermost) StorePost(post mattermost.Post) mattermost.Post {
	if post.ID == "" {
		post.ID = fmt.Sprintf("post-%v", atomic.AddUint64(&s.postIDCounter, 1))
	}
	s.objects.Store(post.ID, post)
	return post
}

func (s *FakeMattermost) UpdatePost(post mattermost.Post) mattermost.Post {
	post = s.StorePost(post)
	s.postUpdates <- post
	return post
}

func (s *FakeMattermost) GetBotUser() mattermost.User {
	user, ok := s.GetUser(s.botUserID)
	if !ok {
		panic("bot user not found")
	}
	return user
}

func (s *FakeMattermost) GetUser(id string) (mattermost.User, bool) {
	if obj, ok := s.objects.Load(id); ok {
		user, ok := obj.(mattermost.User)
		return user, ok
	}
	return mattermost.User{}, false
}

func (s *FakeMattermost) GetUserByEmail(email string) (mattermost.User, bool) {
	if obj, ok := s.objects.Load(fakeUserByEmailKey(email)); ok {
		user, ok := obj.(mattermost.User)
		return user, ok
	}
	return mattermost.User{}, false
}

func (s *FakeMattermost) StoreUser(user mattermost.User) mattermost.User {
	if user.ID == "" {
		user.ID = fmt.Sprintf("user-%v", atomic.AddUint64(&s.userIDCounter, 1))
	}
	s.objects.Store(user.ID, user)
	s.objects.Store(fakeUserByEmailKey(user.Email), user)
	return user
}

func (s *FakeMattermost) GetTeam(id string) (mattermost.Team, bool) {
	if obj, ok := s.objects.Load(id); ok {
		channel, ok := obj.(mattermost.Team)
		return channel, ok
	}
	return mattermost.Team{}, false
}

func (s *FakeMattermost) GetTeamByName(name string) (mattermost.Team, bool) {
	if obj, ok := s.objects.Load(fakeTeamByNameKey(name)); ok {
		channel, ok := obj.(mattermost.Team)
		return channel, ok
	}
	return mattermost.Team{}, false
}

func (s *FakeMattermost) StoreTeam(team mattermost.Team) mattermost.Team {
	if team.ID == "" {
		team.ID = fmt.Sprintf("team-%v", atomic.AddUint64(&s.teamIDCounter, 1))
	}
	s.objects.Store(team.ID, team)
	s.objects.Store(fakeTeamByNameKey(team.Name), team)
	return team
}

func (s *FakeMattermost) GetChannel(id string) (mattermost.Channel, bool) {
	if obj, ok := s.objects.Load(id); ok {
		channel, ok := obj.(mattermost.Channel)
		return channel, ok
	}
	return mattermost.Channel{}, false
}

func (s *FakeMattermost) GetDirectChannelFor(user1, user2 mattermost.User) FakeDirectChannel {
	ids := []string{user1.ID, user2.ID}
	sort.Strings(ids)
	user1ID, user2ID := ids[0], ids[1]
	key := fakeDirectChannelUsersKey{user1ID, user2ID}
	if obj, ok := s.objects.Load(key); ok {
		directChannel, ok := obj.(FakeDirectChannel)
		if !ok {
			panic(fmt.Sprintf("bad channel type %T", obj))
		}
		return directChannel
	}

	channel := s.StoreChannel(mattermost.Channel{})
	directChannel := FakeDirectChannel{
		User1ID: user1ID,
		User2ID: user2ID,
		Channel: channel,
	}
	s.objects.Store(key, directChannel)
	s.objects.Store(fakeDirectChannelKey(channel.ID), directChannel)
	return directChannel
}

func (s *FakeMattermost) GetDirectChannel(id string) (FakeDirectChannel, bool) {
	if obj, ok := s.objects.Load(fakeDirectChannelKey(id)); ok {
		directChannel, ok := obj.(FakeDirectChannel)
		return directChannel, ok
	}
	return FakeDirectChannel{}, false
}

func (s *FakeMattermost) GetChannelByTeamNameAndName(team, name string) (mattermost.Channel, bool) {
	if obj, ok := s.objects.Load(fakeChannelByTeamNameAndNameKey{team: team, channel: name}); ok {
		channel, ok := obj.(mattermost.Channel)
		return channel, ok
	}
	return mattermost.Channel{}, false
}

func (s *FakeMattermost) StoreChannel(channel mattermost.Channel) mattermost.Channel {
	if channel.ID == "" {
		channel.ID = fmt.Sprintf("channel-%v", atomic.AddUint64(&s.channelIDCounter, 1))
	}
	s.objects.Store(channel.ID, channel)

	if channel.TeamID != "" {
		team, ok := s.GetTeam(channel.TeamID)
		if !ok {
			panic(fmt.Sprintf("team id %q is not found", channel.TeamID))
		}
		s.objects.Store(fakeChannelByTeamNameAndNameKey{team: team.Name, channel: channel.Name}, channel)
	}
	return channel
}

func (s *FakeMattermost) CheckNewPost(ctx context.Context) (mattermost.Post, error) {
	select {
	case post := <-s.newPosts:
		return post, nil
	case <-ctx.Done():
		return mattermost.Post{}, trace.Wrap(ctx.Err())
	}
}

func (s *FakeMattermost) CheckPostUpdate(ctx context.Context) (mattermost.Post, error) {
	select {
	case post := <-s.postUpdates:
		return post, nil
	case <-ctx.Done():
		return mattermost.Post{}, trace.Wrap(ctx.Err())
	}
}

func panicIf(err error) {
	if err != nil {
		panic(fmt.Sprintf("%v at %v", err, string(debug.Stack())))
	}
}
