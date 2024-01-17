// Teleport
// Copyright (C) 2023  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package web

import (
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
)

type ListBotsResponse struct {
	// Items is a list of resources retrieved.
	Items []*machineidv1.Bot `json:"items"`
	// StartKey is the position to resume search events.
	StartKey string `json:"startKey"`
}

type CreateBotRequest struct {
	// BotName is the name of the bot
	BotName string `json:"botName"`
	// Roles are the roles that the bot will be able to impersonate
	Roles []string `json:"roles"`
	// Traits are the traits that will be associated with the bot for the purposes of role
	// templating.
	// Where multiple specified with the same name, these will be merged by the
	// server.
	Traits []*machineidv1.Trait `json:"traits"`
}

// listBots returns a paginated list of bots for a given cluster site
func (h *Handler) listBots(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	bots, err := clt.BotServiceClient().ListBots(r.Context(), &machineidv1.ListBotsRequest{
		// todo (mberg) re-evaluate once we agree on a pagination approach
		PageSize:  int32(1000),
		PageToken: "",
	})
	if err != nil {
		return nil, trace.Wrap(err, "error getting bots")
	}

	return ListBotsResponse{
		Items:    bots.Bots,
		StartKey: bots.NextPageToken,
	}, nil
}

// createBot creates a bot
func (h *Handler) createBot(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	var req *CreateBotRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), site)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = clt.BotServiceClient().CreateBot(r.Context(), &machineidv1.CreateBotRequest{
		Bot: &machineidv1.Bot{
			Metadata: &headerv1.Metadata{
				Name: req.BotName,
			},
			Spec: &machineidv1.BotSpec{
				Roles:  req.Roles,
				Traits: req.Traits,
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err, "error creating bot")
	}

	return OK(), nil
}
