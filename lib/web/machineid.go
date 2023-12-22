package web

import (
	"fmt"
	"net/http"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

type CreateGitHubBotRequest struct {
	// BotName is the name of the bot
	BotName string `json:"botName"`
	// BotRoles is a list of roles the bot will have
	BotRoles []string `json:"botRoles"`
	// Repository is the name of the repository from where the workflow is running.
	// This includes the name of the owner e.g `gravitational/teleport`
	Repository string `json:"repository"`
	// Subject is a string that roughly uniquely identifies
	// the workload. The format of this varies depending on the type of
	// github action run.
	Subject string `json:"subject"`
	// RepositoryOwner is name of the organization in which the repository is stored.
	RepositoryOwner string `json:"repositoryOwner"`
	// Workflow is the name of the workflow.
	Workflow string `json:"workflow"`
	// Environment is the name of the environment used by the job.
	Environment string `json:"environment"`
	// Actor is the personal account that initiated the workflow run.
	Actor string `json:"actor"`
	// Ref is the git ref that triggered the workflow run.
	Ref string `json:"ref"`
	// The type of ref, for example: "branch".
	RefType string `json:"refType"`
}

// githubBotCreate creates a GitHub Join Token and a bot using the token
func (h *Handler) gitHubBotCreate(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext) (interface{}, error) {
	var req *CreateGitHubBotRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	ctx := r.Context()

	joinTokenName := fmt.Sprintf("github-token-bot-%s", req.BotName)

	// create token
	token := types.ProvisionTokenV2{
		Kind: types.KindToken,
		// Version: types.V2,
		Metadata: types.Metadata{
			Name: joinTokenName,
		},
		Spec: types.ProvisionTokenSpecV2{
			Roles:      []types.SystemRole{types.RoleBot},
			JoinMethod: types.KindGithub,
			BotName:    req.BotName,
			GitHub: &types.ProvisionTokenSpecV2GitHub{
				Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
					{
						Repository:      req.Repository,
						Sub:             req.Subject,
						RepositoryOwner: req.RepositoryOwner,
						Workflow:        req.Workflow,
						Environment:     req.Environment,
						Actor:           req.Actor,
						Ref:             req.Ref,
						RefType:         req.RefType,
					},
				},
			},
		},
	}

	clt, err := sctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := clt.CreateToken(ctx, &token); err != nil {
		return nil, trace.Wrap(err)
	}

	// create bot
	clt.CreateBot(r.Context(), &proto.CreateBotRequest{
		Name:    req.BotName,
		Roles:   req.BotRoles,
		TokenID: joinTokenName,
		// TODO: ttl
	})

	return nil, nil
}
