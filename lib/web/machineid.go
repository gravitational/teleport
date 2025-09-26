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
	"strconv"
	"strings"
	"time"

	yaml "github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	tslices "github.com/gravitational/teleport/lib/utils/slices"
)

const (
	// webUIFlowBotGitHubActionsSSH is the value of the webUIFlowLabelKey
	// added to a resource created via the Bot GitHub Actions web UI flow.
	webUIFlowBotGitHubActionsSSH = "github-actions-ssh"
)

type ListBotsResponse struct {
	// Items is a list of resources retrieved.
	Items []*machineidv1.Bot `json:"items"`
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

// listBots returns a list of bots for a given cluster site. It does not leverage pagination from the UI. Due to the
// nature of the bot:user relationship, pagination is not yet supported. This endpoint will return all bots.
func (h *Handler) listBots(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var items []*machineidv1.Bot
	for pageToken := ""; ; {
		bots, err := clt.BotServiceClient().ListBots(r.Context(), &machineidv1.ListBotsRequest{
			PageSize:  int32(1000),
			PageToken: pageToken,
		})
		// todo (michellescripts) consider returning partial results
		if err != nil {
			return nil, trace.Wrap(err, "error getting bots")
		}
		items = append(items, bots.Bots...)
		pageToken = bots.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return ListBotsResponse{
		Items: items,
	}, nil
}

// createBot creates a bot
func (h *Handler) createBot(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	var req *CreateBotRequest
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = clt.BotServiceClient().CreateBot(r.Context(), &machineidv1.CreateBotRequest{
		Bot: &machineidv1.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: req.BotName,
				Labels: map[string]string{
					webUIFlowLabelKey: webUIFlowBotGitHubActionsSSH,
				},
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

func (h *Handler) deleteBot(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	name := params.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("missing bot name")
	}

	_, err = clt.BotServiceClient().DeleteBot(r.Context(), &machineidv1.DeleteBotRequest{BotName: name})
	if err != nil {
		return nil, trace.Wrap(err, "error deleting bot")
	}

	return OK(), nil
}

// CreateBotJoinTokenRequest represents a client request to
// create a bot join token
type CreateBotJoinTokenRequest struct {
	// IntegrationName is the name attributed to the bot integration, which
	// is used to name the resources created during the UI flow.
	IntegrationName string `json:"integrationName"`
	// JoinMethod is the joining method required in order to use this token.
	JoinMethod types.JoinMethod `json:"joinMethod"`
	// GitHub allows the configuration of options specific to the "github" join method.
	GitHub *types.ProvisionTokenSpecV2GitHub `json:"gitHub"`
	// WebFlowLabel is the value of the label attributed to bots created via the web UI
	WebFlowLabel string `json:"webFlowLabel"`
}

// createBotJoinToken creates a bot join token
func (h *Handler) createBotJoinToken(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	var req *CreateBotJoinTokenRequest
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := types.ValidateJoinMethod(req.JoinMethod); err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	spec := types.ProvisionTokenSpecV2{
		Roles:      []types.SystemRole{types.RoleBot},
		JoinMethod: req.JoinMethod,
		GitHub:     req.GitHub,
		BotName:    req.IntegrationName,
	}
	provisionToken, err := types.NewProvisionTokenFromSpec(req.IntegrationName, time.Time{}, spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	provisionToken.SetLabels(map[string]string{
		webUIFlowLabelKey: req.WebFlowLabel,
	})

	err = clt.CreateToken(r.Context(), provisionToken)
	if err != nil {
		return nil, trace.Wrap(err, "error creating join token")
	}

	return &nodeJoinToken{
		ID:     provisionToken.GetName(),
		Expiry: provisionToken.Expiry(),
		Method: provisionToken.GetJoinMethod(),
	}, nil
}

// getBot retrieves a bot by name
func (h *Handler) getBot(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	botName := p.ByName("name")
	if botName == "" {
		return nil, trace.BadParameter("empty name")
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bot, err := clt.BotServiceClient().GetBot(r.Context(), &machineidv1.GetBotRequest{
		BotName: botName,
	})
	if err != nil {
		return nil, trace.Wrap(err, "error querying bot")
	}

	return bot, nil
}

// updateBot updates a bot with provided roles. The only supported change via this endpoint today is roles.
// TODO(nicholasmarais1158) DELETE IN v20.0.0 - replaced by updateBotV2
// MUST delete with related code found in `web/packages/teleport/src/services/bot/bot.ts`
func (h *Handler) updateBot(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	var request updateBotRequest
	if err := httplib.ReadResourceJSON(r, &request); err != nil {
		return nil, trace.Wrap(err)
	}

	botName := p.ByName("name")
	if botName == "" {
		return nil, trace.BadParameter("empty name")
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mask, err := fieldmaskpb.New(&machineidv1.Bot{}, "spec.roles")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := clt.BotServiceClient().UpdateBot(r.Context(), &machineidv1.UpdateBotRequest{
		UpdateMask: mask,
		Bot: &machineidv1.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: botName,
			},
			Spec: &machineidv1.BotSpec{
				Roles: request.Roles,
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err, "unable to find existing bot")
	}

	return updated, nil
}

type updateBotRequest struct {
	Roles []string `json:"roles"`
}

// updateBotV2 updates a bot with provided roles, traits and max_session_ttl.
func (h *Handler) updateBotV2(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	var request updateBotRequestV2
	if err := httplib.ReadResourceJSON(r, &request); err != nil {
		return nil, trace.Wrap(err)
	}

	botName := p.ByName("name")
	if botName == "" {
		return nil, trace.BadParameter("empty name")
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mask, err := fieldmaskpb.New(&machineidv1.Bot{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	spec := machineidv1.BotSpec{}

	if request.Roles != nil {
		mask.Append(&machineidv1.Bot{}, "spec.roles")

		spec.Roles = request.Roles
	}

	if request.Traits != nil {
		mask.Append(&machineidv1.Bot{}, "spec.traits")

		traits := make([]*machineidv1.Trait, len(request.Traits))
		for i, trait := range request.Traits {
			traits[i] = &machineidv1.Trait{
				Name:   trait.Name,
				Values: trait.Values,
			}
		}

		spec.Traits = traits
	}

	if request.MaxSessionTtl != "" {
		mask.Append(&machineidv1.Bot{}, "spec.max_session_ttl")

		ttl, err := time.ParseDuration(request.MaxSessionTtl)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		spec.MaxSessionTtl = durationpb.New(ttl)
	}

	updated, err := clt.BotServiceClient().UpdateBot(r.Context(), &machineidv1.UpdateBotRequest{
		UpdateMask: mask,
		Bot: &machineidv1.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: botName,
			},
			Spec: &spec,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err, "unable to find existing bot")
	}

	return updated, nil
}

type updateBotRequestV2 struct {
	Roles         []string                `json:"roles"`
	Traits        []updateBotRequestTrait `json:"traits"`
	MaxSessionTtl string                  `json:"max_session_ttl"`
}

type updateBotRequestTrait struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// getBotInstance retrieves a bot instance by id
func (h *Handler) getBotInstance(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	botName := p.ByName("name")
	instanceId := p.ByName("id")
	if botName == "" {
		return nil, trace.BadParameter("empty bot name")
	}
	if instanceId == "" {
		return nil, trace.BadParameter("empty id")
	}

	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	instance, err := clt.BotInstanceServiceClient().GetBotInstance(r.Context(), &machineidv1.GetBotInstanceRequest{
		InstanceId: instanceId,
		BotName:    botName,
	})
	if err != nil {
		return nil, trace.Wrap(err, "error querying bot instance")
	}

	yaml, err := yaml.Marshal(types.ProtoResource153ToLegacy(instance))
	if err != nil {
		return nil, trace.Wrap(err, "error stringifying to yaml")
	}

	return GetBotInstanceResponse{
		BotInstance: instance,
		YAML:        string(yaml),
	}, nil
}

type GetBotInstanceResponse struct {
	BotInstance *machineidv1.BotInstance `json:"bot_instance"`
	YAML        string                   `json:"yaml"`
}

// listBotInstances returns a list of bot instances for a given cluster site.
func (h *Handler) listBotInstances(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var pageSize int64 = 20
	if r.URL.Query().Has("page_size") {
		pageSize, err = strconv.ParseInt(r.URL.Query().Get("page_size"), 10, 32)
		if err != nil {
			return nil, trace.BadParameter("invalid page size")
		}
	}

	var sort *types.SortBy
	if r.URL.Query().Has("sort") {
		sortString := r.URL.Query().Get("sort")
		s := types.GetSortByFromString(sortString)
		sort = &s
	}

	//nolint:staticcheck // SA1019. Kept for backward compatibility.
	instances, err := clt.BotInstanceServiceClient().ListBotInstances(r.Context(), &machineidv1.ListBotInstancesRequest{
		FilterBotName:    r.URL.Query().Get("bot_name"),
		PageSize:         int32(pageSize),
		PageToken:        r.URL.Query().Get("page_token"),
		FilterSearchTerm: r.URL.Query().Get("search"),
		Sort:             sort,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiInstances := tslices.Map(instances.BotInstances, func(instance *machineidv1.BotInstance) BotInstance {
		heartbeat := services.GetBotInstanceLatestHeartbeat(instance)

		uiInstance := BotInstance{
			InstanceId: instance.Spec.InstanceId,
			BotName:    instance.Spec.BotName,
		}

		if heartbeat != nil {
			uiInstance.JoinMethodLatest = heartbeat.JoinMethod
			uiInstance.HostNameLatest = heartbeat.Hostname
			uiInstance.VersionLatest = heartbeat.Version
			uiInstance.ActiveAtLatest = heartbeat.RecordedAt.AsTime().Format(time.RFC3339)
			uiInstance.OSLatest = heartbeat.Os
		}

		return uiInstance
	})

	return ListBotInstancesResponse{
		BotInstances:  uiInstances,
		NextPageToken: instances.NextPageToken,
	}, nil
}

// listBotInstancesV2 returns a list of bot instances for a given cluster site.
func (h *Handler) listBotInstancesV2(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, sctx *SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	request := &machineidv1.ListBotInstancesV2Request{
		PageToken: r.URL.Query().Get("page_token"),
		SortField: r.URL.Query().Get("sort_field"),
		Filter: &machineidv1.ListBotInstancesV2Request_Filters{
			BotName:    r.URL.Query().Get("bot_name"),
			SearchTerm: r.URL.Query().Get("search"),
			Query:      r.URL.Query().Get("query"),
		},
	}

	if r.URL.Query().Has("page_size") {
		pageSize, err := strconv.ParseInt(r.URL.Query().Get("page_size"), 10, 32)
		if err != nil {
			return nil, trace.BadParameter("invalid page size")
		}
		request.PageSize = int32(pageSize)
	}

	if r.URL.Query().Has("sort_dir") {
		sortDir := r.URL.Query().Get("sort_dir")
		request.SortDesc = strings.ToLower(sortDir) == "desc"
	}

	instances, err := clt.BotInstanceServiceClient().ListBotInstancesV2(r.Context(), request)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiInstances := tslices.Map(instances.BotInstances, func(instance *machineidv1.BotInstance) BotInstance {
		heartbeat := services.GetBotInstanceLatestHeartbeat(instance)
		authentication := services.GetBotInstanceLatestAuthentication(instance)

		uiInstance := BotInstance{
			InstanceId: instance.GetSpec().GetInstanceId(),
			BotName:    instance.GetSpec().GetBotName(),
		}

		if authentication != nil {
			uiInstance.JoinMethodLatest = authentication.GetJoinMethod()
		}

		if heartbeat != nil {
			uiInstance.HostNameLatest = heartbeat.GetHostname()
			uiInstance.VersionLatest = heartbeat.GetVersion()
			uiInstance.ActiveAtLatest = heartbeat.GetRecordedAt().AsTime().Format(time.RFC3339)
			uiInstance.OSLatest = heartbeat.GetOs()
		}

		return uiInstance
	})

	return ListBotInstancesResponse{
		BotInstances:  uiInstances,
		NextPageToken: instances.NextPageToken,
	}, nil
}

type ListBotInstancesResponse struct {
	BotInstances  []BotInstance `json:"bot_instances"`
	NextPageToken string        `json:"next_page_token,omitempty"`
}

type BotInstance struct {
	InstanceId       string `json:"instance_id"`
	BotName          string `json:"bot_name"`
	JoinMethodLatest string `json:"join_method_latest,omitempty"`
	HostNameLatest   string `json:"host_name_latest,omitempty"`
	VersionLatest    string `json:"version_latest,omitempty"`
	ActiveAtLatest   string `json:"active_at_latest,omitempty"`
	OSLatest         string `json:"os_latest,omitempty"`
}
