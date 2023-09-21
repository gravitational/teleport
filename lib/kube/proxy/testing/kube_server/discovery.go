/*
Copyright 2023 Gravitational, Inc.

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
