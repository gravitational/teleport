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

package kubeserver

import (
	_ "embed"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

var (
	//go:embed data/api.json
	apiResponse string
	//go:embed data/api_v1.json
	apiV1Response string
	//go:embed data/apis.json
	apisResponse string
	//go:embed data/api_teleport.json
	teleportAPIResponse string
)

const (
	apiEndpoint         = "/api"
	apiV1Endpoint       = "/api/v1"
	apisEndpoint        = "/apis"
	teleportAPIEndpoint = "/apis/resources.teleport.dev/v6"
)

func (s *KubeMockServer) discoveryEndpoint(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	switch req.URL.Path {
	case apiEndpoint:
		w.Write([]byte(apiResponse))
		return nil, nil
	case apiV1Endpoint:
		w.Write([]byte(apiV1Response))
		return nil, nil
	case apisEndpoint:
		w.Write([]byte(apisResponse))
		return nil, nil
	case teleportAPIEndpoint:
		w.Write([]byte(teleportAPIResponse))
		return nil, nil
	default:
		return nil, trace.NotFound("path %v is not supported", req.URL.Path)
	}
}
