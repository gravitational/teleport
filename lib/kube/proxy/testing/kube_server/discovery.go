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
	"cmp"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/lib/httplib"
)

var (
	//go:embed data/api.json
	apiResponse string
	//go:embed data/api_v1.json
	apiV1Response string
	//go:embed data/apis_rbac.authorization.k8s.io_v1.json
	apiRBACV1Response string
)

const (
	apiEndpoint     = "/api"
	apiV1Endpoint   = "/api/v1"
	apisEndpoint    = "/apis"
	apiRBACEndpoint = "/apis/rbac.authorization.k8s.io/v1"
)

func (s *KubeMockServer) discoveryEndpoint(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	switch req.URL.Path {
	case apiEndpoint:
		w.Write([]byte(apiResponse))
		return nil, nil
	case apiV1Endpoint:
		w.Write([]byte(apiV1Response))
		return nil, nil
	case apiRBACEndpoint:
		w.Write([]byte(apiRBACV1Response))
		return nil, nil
	case apisEndpoint:
		w.Write(apisDiscovery(s.crds))
		return nil, nil
	default:
		return nil, trace.NotFound("path %v is not supported", req.URL.Path)
	}
}

func apisDiscovery(crds map[GVP]*CRD) []byte {
	byGroup := map[string][]*CRD{}
	for _, crd := range crds {
		byGroup[crd.group] = append(byGroup[crd.group], crd)
	}
	for _, crds := range byGroup {
		slices.SortFunc(crds, func(a, b *CRD) int { return cmp.Compare(a.version, b.version) })
	}

	type (
		version struct {
			GroupVersion string `json:"groupVersion"`
			Version      string `json:"version"`
		}
		group struct {
			Name             string    `json:"name"`
			Versions         []version `json:"versions"`
			PreferredVersion version   `json:"preferredVersion"`
		}
		discovery struct {
			Kind       string  `json:"kind"`
			APIVersion string  `json:"apiVersion"`
			Groups     []group `json:"groups"`
		}
	)

	out := discovery{
		Kind:       "APIGroupList",
		APIVersion: "v1",
		Groups: []group{
			{
				Name: "rbac.authorization.k8s.io",
				PreferredVersion: version{
					GroupVersion: "rbac.authorization.k8s.io/v1",
					Version:      "v1",
				},
				Versions: []version{
					{
						GroupVersion: "rbac.authorization.k8s.io/v1",
						Version:      "v1",
					},
				},
			},
		},
	}

	for groupName, crds := range byGroup {
		g := group{
			Name: groupName,
			PreferredVersion: version{
				GroupVersion: groupName + "/" + crds[0].version,
				Version:      crds[0].version,
			},
		}
		for _, crd := range crds {
			g.Versions = append(g.Versions, version{
				GroupVersion: groupName + "/" + crd.version,
				Version:      crd.version,
			})
		}
		out.Groups = append(out.Groups, g)
	}

	buf, _ := json.Marshal(out) // Can't fail.
	return buf
}

func crdDiscovery(crd *CRD) httplib.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
		_, err := fmt.Fprintf(w, `{
			"kind": "APIResourceList",
			"apiVersion": "v1",
			"groupVersion": "%s/%s",
			"resources": [
			  {
			    "name": "%s",
			    "singularName": "%s",
			    "namespaced": %t,
			    "kind": "%s",
			    "group": "%s",
			    "version": "%s",
			    "verbs": [
			      "delete",
			      "deletecollection",
			      "get",
			      "list",
			      "patch",
			      "create",
			      "update",
			      "watch"
			    ],
			    "storageVersionHash": ""
			  },
			  {
			    "name": "%s/status",
			    "singularName": "%s",
			    "namespaced": %t,
			    "kind": "%s",
			    "group": "%s",
			    "version": "%s",
			    "verbs": [
			      "get",
			      "patch",
			      "update"
			    ]
			  }
			]
		      }`,
			crd.group,
			crd.version,
			crd.plural,
			strings.ToLower(crd.kind),
			crd.namespaced,
			crd.kind,
			crd.group,
			crd.version,
			crd.plural,
			strings.ToLower(crd.kind),
			crd.namespaced,
			crd.kind,
			crd.group,
			crd.version,
		)
		return nil, err
	}
}
