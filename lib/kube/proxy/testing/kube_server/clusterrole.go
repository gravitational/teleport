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
	"net/http"
	"sort"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	authv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
)

var clusterRoleList = authv1.ClusterRoleList{
	TypeMeta: metav1.TypeMeta{
		Kind:       "ClusterRoleList",
		APIVersion: "rbac.authorization.k8s.io/v1",
	},
	ListMeta: metav1.ListMeta{
		ResourceVersion: "1231415",
	},
	Items: []authv1.ClusterRole{
		newClusterRole("cr-nginx-1"),
		newClusterRole("cr-nginx-2"),
		newClusterRole("cr-test"),
	},
}

func newClusterRole(name string) authv1.ClusterRole {
	return authv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func (s *KubeMockServer) listClusterRoles(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	return &clusterRoleList, nil
}

func (s *KubeMockServer) getClusterRole(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	name := p.ByName("name")
	filter := func(role authv1.ClusterRole) bool {
		return role.Name == name
	}
	for _, role := range clusterRoleList.Items {
		if filter(role) {
			return role, nil
		}
	}
	return nil, trace.NotFound("cluster role %q not found", name)
}

func (s *KubeMockServer) deleteClusterRole(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	name := p.ByName("name")
	deleteOpts, err := parseDeleteCollectionBody(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reqID := ""
	if deleteOpts.Preconditions != nil && deleteOpts.Preconditions.UID != nil {
		reqID = string(*deleteOpts.Preconditions.UID)
	}
	filter := func(role authv1.ClusterRole) bool {
		return role.Name == name
	}
	for _, role := range clusterRoleList.Items {
		if filter(role) {
			s.mu.Lock()
			key := deletedResource{reqID, types.KindKubeClusterRole}
			s.deletedResources[key] = append(s.deletedResources[key], name)
			s.mu.Unlock()
			return role, nil
		}
	}
	return nil, trace.NotFound("cluster %q not found", name)
}

func (s *KubeMockServer) DeletedClusterRoles(reqID string) []string {
	s.mu.Lock()
	key := deletedResource{reqID, types.KindKubeClusterRole}
	deleted := make([]string, len(s.deletedResources[key]))
	copy(deleted, s.deletedResources[key])
	s.mu.Unlock()
	sort.Strings(deleted)
	return deleted
}
