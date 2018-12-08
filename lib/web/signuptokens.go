/*
Copyright 2018 Gravitational, Inc.

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

package web

import (
	"net/http"
	"time"

	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"

	"github.com/julienschmidt/httprouter"
)

type createSignupTokenRequest struct {
	Username      string        `json:"username"`
	AllowedLogins []string      `json:"allowed_logins"`
	KubeGroups    []string      `json:"kube_groups"`
	TTL           time.Duration `json:"ttl"`
}

type createSignupTokenResponse struct {
	Token string `json:"token"`
}

func (h *Handler) createSignupToken(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	var req *createSignupTokenRequest
	err := httplib.ReadJSON(r, &req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get a site specific auth client.
	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create user signup token.
	user := services.UserV1{
		Name:          req.Username,
		AllowedLogins: req.AllowedLogins,
		KubeGroups:    req.KubeGroups,
	}
	token, err := clt.CreateSignupToken(user, req.TTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return createSignupTokenResponse{
		Token: token,
	}, nil
}

type getSignupTokensResponse struct {
	Tokens []services.SignupToken `json:"tokens"`
}

func (h *Handler) getSignupTokens(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	// Get a site specific auth client.
	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Get all user signup tokens.
	tokens, err := clt.GetSignupTokens()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &getSignupTokensResponse{
		Tokens: tokens,
	}, nil
}

func (h *Handler) deleteSignupToken(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	// Get a site specific auth client.
	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Remove specific user signup token.
	err = clt.DeleteToken(p.ByName("token"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ok(), nil
}
