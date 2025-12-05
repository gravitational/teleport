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
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/tfgen"
)

type terraformStringifyResponse struct {
	Terraform string `json:"terraform"`
}

func (h *Handler) terraformStringify(
	w http.ResponseWriter,
	r *http.Request,
	params httprouter.Params,
	ctx *SessionContext,
) (any, error) {
	kind := params.ByName("kind")
	if len(kind) == 0 {
		return nil, trace.BadParameter("query param %q is required", "kind")
	}

	var tfResult []byte

	switch kind {
	case types.KindAccessMonitoringRule:
		var req struct {
			Resource *accessmonitoringrulesv1.AccessMonitoringRule `json:"resource"`
		}

		err := httplib.ReadResourceJSON(r, &req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		tfResult, err = tfgen.Generate(req.Resource)
		if err != nil {
			return nil, trace.Wrap(err)
		}

	default:
		return nil, trace.NotImplemented("Generate Terraform for kind %q is not supported", kind)
	}

	return terraformStringifyResponse{Terraform: string(tfResult)}, nil
}
