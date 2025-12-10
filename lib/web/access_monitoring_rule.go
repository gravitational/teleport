/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"encoding/json"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/lib/tfgen"
)

func (h *Handler) accessMonitoringRuleGenerateTerraform(
	_ http.ResponseWriter,
	r *http.Request,
	_ httprouter.Params,
	_ *SessionContext,
) (any, error) {
	var req accessMonitoringRuleGenerateTerraformRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, trace.Wrap(err)
	}

	tfResult, err := tfgen.Generate(req.Resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return accessMonitoringRuleGenerateTerraformResponse{Terraform: string(tfResult)}, nil
}

type accessMonitoringRuleGenerateTerraformRequest struct {
	Resource *accessmonitoringrulesv1.AccessMonitoringRule `json:"resource"`
}

type accessMonitoringRuleGenerateTerraformResponse struct {
	Terraform string `json:"terraform"`
}
