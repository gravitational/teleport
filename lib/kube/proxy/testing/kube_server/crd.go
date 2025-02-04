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
	"path/filepath"
	"sort"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var teleportRoleList = metav1.List{
	TypeMeta: metav1.TypeMeta{
		Kind:       "TeleportRoleList",
		APIVersion: "resources.teleport.dev/v6",
	},
	ListMeta: metav1.ListMeta{
		ResourceVersion: "1231415",
	},
	Items: []runtime.RawExtension{
		{
			Object: newTeleportRole("telerole-1", "default"),
		},
		{
			Object: newTeleportRole("telerole-1", "default"),
		},
		{
			Object: newTeleportRole("telerole-2", "default"),
		},
		{
			Object: newTeleportRole("telerole-test", "default"),
		},
		{
			Object: newTeleportRole("telerole-1", "dev"),
		},
		{
			Object: newTeleportRole("telerole-2", "dev"),
		},
	},
}

func newTeleportRole(name, namespace string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetKind("TeleportRole")
	obj.SetAPIVersion("resources.teleport.dev/v6")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	return obj
}

func (s *KubeMockServer) listTeleportRoles(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	items := []runtime.RawExtension{}

	namespace := p.ByName("namespace")
	filter := func(obj runtime.Object) bool {
		objNamespace := obj.(*unstructured.Unstructured).GetNamespace()
		return len(namespace) == 0 || namespace == objNamespace
	}
	for _, obj := range teleportRoleList.Items {
		if filter(obj.Object) {
			items = append(items, obj)
		}
	}
	return metav1.List{
		TypeMeta: metav1.TypeMeta{
			Kind:       "TeleportRoleList",
			APIVersion: "resources.teleport.dev/v6",
		},
		ListMeta: metav1.ListMeta{
			ResourceVersion: "1231415",
		},
		Items: items,
	}, nil
}

func (s *KubeMockServer) getTeleportRole(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	namespace := p.ByName("namespace")
	name := p.ByName("name")
	filter := func(obj runtime.Object) bool {
		metaObj := obj.(*unstructured.Unstructured)
		return metaObj.GetName() == name && namespace == metaObj.GetNamespace()
	}
	for _, obj := range teleportRoleList.Items {
		if filter(obj.Object) {
			return obj.Object, nil
		}
	}
	return nil, trace.NotFound("teleport %q not found", filepath.Join(namespace, name))
}

const (
	teleportRoleKind = "TeleportRole"
)

func (s *KubeMockServer) deleteTeleportRole(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	namespace := p.ByName("namespace")
	name := p.ByName("name")
	deleteOpts, err := parseDeleteCollectionBody(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reqID := ""
	if deleteOpts.Preconditions != nil && deleteOpts.Preconditions.UID != nil {
		reqID = string(*deleteOpts.Preconditions.UID)
	}
	filter := func(obj runtime.Object) bool {
		metaObj := obj.(*unstructured.Unstructured)
		return metaObj.GetName() == name && namespace == metaObj.GetNamespace()
	}
	for _, obj := range teleportRoleList.Items {
		if filter(obj.Object) {
			s.mu.Lock()
			s.deletedResources[deletedResource{kind: teleportRoleKind, requestID: reqID}] = append(s.deletedResources[deletedResource{kind: teleportRoleKind, requestID: reqID}], filepath.Join(namespace, name))
			s.mu.Unlock()
			return obj.Object, nil
		}
	}
	return nil, trace.NotFound("teleportrole %q not found", filepath.Join(namespace, name))
}

func (s *KubeMockServer) DeletedTeleportRoles(reqID string) []string {
	s.mu.Lock()
	key := deletedResource{kind: teleportRoleKind, requestID: reqID}
	deleted := make([]string, len(s.deletedResources[key]))
	copy(deleted, s.deletedResources[key])
	s.mu.Unlock()
	sort.Strings(deleted)
	return deleted
}
