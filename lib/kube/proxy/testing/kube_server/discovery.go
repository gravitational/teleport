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
	apiEndpoint string
	//go:embed data/api_v1.json
	apiEndpointV1 string
	//go:embed data/apis.json
	apisResponse string
	//go:embed data/api_teleport.json
	teleportResource string
)

func (s *KubeMockServer) discoveryEndpoint(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	switch req.URL.Path {
	case "/api":
		w.Write([]byte(apiEndpoint))
		return nil, nil
	case "/api/v1":
		w.Write([]byte(apiEndpointV1))
		return nil, nil
	case "/apis":
		w.Write([]byte(apisResponse))
		return nil, nil
	case "/apis/resources.teleport.dev/v6":
		w.Write([]byte(teleportResource))
		return nil, nil
	default:
		return nil, trace.NotFound("path %v is not supported", req.URL.Path)
	}
}
