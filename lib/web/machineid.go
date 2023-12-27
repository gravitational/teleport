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

type CreateGitHubBotRequest struct {
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

// createBot creates a bot
func (h *Handler) createBot(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, site reversetunnelclient.RemoteSite) (interface{}, error) {
	var req *CreateGitHubBotRequest
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
				Traits: []*machineidv1.Trait{},
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err, "error creating bot")
	}

	return OK(), err
}
