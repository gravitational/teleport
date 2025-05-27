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

package kubeserver

import (
	"net/http"
	"sort"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
)

var namespaceList = corev1.NamespaceList{
	TypeMeta: metav1.TypeMeta{
		Kind:       "NamespaceList",
		APIVersion: "v1",
	},
	ListMeta: metav1.ListMeta{
		ResourceVersion: "1231415",
	},
	Items: []corev1.Namespace{
		newNamespace("default"),
		newNamespace("test"),
		newNamespace("dev"),
		newNamespace("prod"),
	},
}

func newNamespace(name string) corev1.Namespace {
	return corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func (s *KubeMockServer) listNamespaces(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	list := namespaceList.DeepCopy()
	return list, nil
}

func (s *KubeMockServer) getNamespace(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	name := p.ByName("name")
	filter := func(role corev1.Namespace) bool {
		return role.Name == name
	}
	for _, role := range namespaceList.Items {
		if filter(role) {
			return role, nil
		}
	}
	return nil, trace.NotFound("namespace %q not found", name)
}

func (s *KubeMockServer) deleteNamespace(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	name := p.ByName("name")

	deleteOpts, err := parseDeleteCollectionBody(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reqID := ""
	if deleteOpts.Preconditions != nil && deleteOpts.Preconditions.UID != nil {
		reqID = string(*deleteOpts.Preconditions.UID)
	}
	filter := func(role corev1.Namespace) bool {
		return role.Name == name
	}
	for _, role := range namespaceList.Items {
		if filter(role) {
			s.mu.Lock()
			key := deletedResource{reqID, types.KindKubeNamespace}
			s.deletedResources[key] = append(s.deletedResources[key], name)
			s.mu.Unlock()
			return role, nil
		}
	}
	return nil, trace.NotFound("namespace %q not found", name)
}

func (s *KubeMockServer) Deletednamespaces(reqID string) []string {
	s.mu.Lock()
	key := deletedResource{reqID, types.KindKubeNamespace}
	deleted := make([]string, len(s.deletedResources[key]))
	copy(deleted, s.deletedResources[key])
	s.mu.Unlock()
	sort.Strings(deleted)
	return deleted
}
