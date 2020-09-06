/*
Copyright 2020 Gravitational, Inc.

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

package app

import (
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/gravitational/trace"
)

type session struct {
}

type sessionCache struct {
}

func newSessionCache() (*sessionCache, error) {
	return &sessionCache{}, nil
}

func (s *sessionCache) get(cookie *Cookie) (*session, error) {
	//// TODO(russjones): Extract session cookie.
	//session, err := h.c.AuthClient.GetAppSession(r.Context(), services.GetAppSessionRequest{
	//	Username:   "",
	//	ParentHash: "",
	//	SessionID:  "",
	//})

	return &session{}, nil
}

type Cookie struct {
	Username   string `json:"username"`
	ParentHash string `json:"parent_hash"`
	SessionID  string `json:"session_id"`
}

func extractCookie(r *http.Request) (*Cookie, error) {
	rawCookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if rawCookie != nil && rawCookie.Value == "" {
		return nil, trace.BadParameter("cookie missing")
	}

	cookie, err := decodeCookie(rawCookie.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cookie, nil
}

func decodeCookie(cookieValue string) (*Cookie, error) {
	cookieBytes, err := hex.DecodeString(cookieValue)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var cookie Cookie
	if err := json.Unmarshal(cookieBytes, &cookie); err != nil {
		return nil, trace.Wrap(err)
	}
	return &cookie, nil
}

func encodeCookie(cookie *Cookie) (string, error) {
	bytes, err := json.Marshal(cookie)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return hex.EncodeToString(bytes), nil
}

const (
	cookieName = "app.session"
)
