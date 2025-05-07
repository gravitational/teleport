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

package web

import (
	"net/http"

	yaml "github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
)

type yamlParseRequest struct {
	YAML string `json:"yaml"`
}

type yamlParseResponse struct {
	Resource interface{} `json:"resource"`
}

type yamlStringifyResponse struct {
	YAML string `json:"yaml"`
}

func (h *Handler) yamlParse(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	kind := params.ByName("kind")
	if len(kind) == 0 {
		return nil, trace.BadParameter("query param %q is required", "kind")
	}

	var req yamlParseRequest
	if err := httplib.ReadResourceJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	switch kind {
	case types.KindAccessMonitoringRule:
		resource, err := yamlToAccessMonitoringRuleResource(req.YAML)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return yamlParseResponse{Resource: resource}, nil

	case types.KindRole:
		resource, err := yamlToRole(req.YAML)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return yamlParseResponse{Resource: resource}, nil

	case types.KindToken:
		resource, err := yamlToProvisionToken(req.YAML)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return yamlParseResponse{Resource: resource}, nil

	default:
		return nil, trace.NotImplemented("parsing YAML for kind %q is not supported", kind)
	}
}

func (h *Handler) yamlStringify(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	kind := params.ByName("kind")
	if len(kind) == 0 {
		return nil, trace.BadParameter("query param %q is required", "kind")
	}

	var resource interface{}

	switch kind {
	case types.KindAccessMonitoringRule:
		var req struct {
			Resource *accessmonitoringrulesv1.AccessMonitoringRule `json:"resource"`
		}
		if err := httplib.ReadResourceJSON(r, &req); err != nil {
			return nil, trace.Wrap(err)
		}
		resource = req.Resource

	case types.KindRole:
		var req struct {
			Resource types.RoleV6 `json:"resource"`
		}
		if err := httplib.ReadJSON(r, &req); err != nil {
			return nil, trace.Wrap(err)
		}
		if err := req.Resource.CheckAndSetDefaults(); err != nil {
			return nil, err
		}
		resource = req.Resource

	default:
		return nil, trace.NotImplemented("YAML stringifying for kind %q is not supported", kind)
	}

	data, err := yaml.Marshal(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return yamlStringifyResponse{YAML: string(data)}, nil
}

func yamlToAccessMonitoringRuleResource(yaml string) (*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	extractedRes, err := extractResource(yaml)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if extractedRes.Kind != types.KindAccessMonitoringRule {
		return nil, trace.BadParameter("resource kind %q is invalid, only acces_monitoring_rule is allowed", extractedRes.Kind)
	}
	resource, err := services.UnmarshalAccessMonitoringRule(extractedRes.Raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resource, nil
}

func yamlToRole(yaml string) (types.Role, error) {
	extractedRes, err := extractResource(yaml)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if extractedRes.Kind != types.KindRole {
		return nil, trace.BadParameter("resource kind %q is invalid, only role is allowed", extractedRes.Kind)
	}
	resource, err := services.UnmarshalRole(extractedRes.Raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resource, nil
}

func yamlToProvisionToken(yaml string) (types.ProvisionToken, error) {
	extractedRes, err := extractResource(yaml)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if extractedRes.Kind != types.KindToken {
		return nil, trace.BadParameter("resource kind %q is invalid, only token is allowed", extractedRes.Kind)
	}
	resource, err := services.UnmarshalProvisionToken(extractedRes.Raw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resource, nil
}
