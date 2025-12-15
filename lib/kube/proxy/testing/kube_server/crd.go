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
	"path"
	"path/filepath"
	"sort"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gravitational/teleport/lib/httplib"
)

// GVP is a group, version, and plural tuple.
type GVP struct{ group, version, plural string }

// CRD is a custom resource definition with it's resources.
type CRD struct {
	*unstructured.Unstructured
	GVP
	kind       string
	listKind   string
	namespaced bool
	items      []runtime.RawExtension
}

// Copy the CRD.
func (c CRD) Copy() *CRD {
	cpy := c
	cpy.Unstructured = cpy.Unstructured.DeepCopy()
	return &cpy
}

func (c CRD) GetKindPlural() string {
	return c.GVP.plural
}

func NewTeleportRoleCRD() *CRD {
	return NewCRD(
		"resources.teleport.dev",
		"v6",
		"teleportroles",
		"TeleportRole",
		"TeleportRoleList",
		true,
	)
}

var WithTeleportRoleCRD = WithCRD(
	NewTeleportRoleCRD(),
	NewObject("default", "telerole-1"),
	NewObject("default", "telerole-1"), // Intentional duplicate.
	NewObject("default", "telerole-2"),
	NewObject("default", "telerole-test"),
	NewObject("dev", "telerole-1"),
	NewObject("dev", "telerole-2"),
)

func NewObject(namespace, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetNamespace(namespace)
	obj.SetName(name)
	return obj
}

func NewCRD(group, version, plural, kind, listKind string, namespaced bool) *CRD {
	obj := &unstructured.Unstructured{}
	obj.SetKind(kind)
	obj.SetAPIVersion(group + "/" + version)
	return &CRD{
		Unstructured: obj,
		GVP:          GVP{group, version, plural},
		kind:         kind,
		listKind:     listKind,
		namespaced:   namespaced,
	}
}

func (s *KubeMockServer) deleteCRD(crd *CRD) httplib.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
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
			namer, ok1 := obj.(interface{ GetName() string })
			if !ok1 || namer.GetName() != name {
				return false
			}
			nser, ok2 := obj.(interface{ GetNamespace() string })
			return namespace == "" || (ok2 && nser.GetNamespace() == namespace)
		}
		dr := deletedResource{kind: crd.kind, requestID: reqID}
		for _, obj := range crd.items {
			if filter(obj.Object) {
				s.mu.Lock()
				s.deletedResources[dr] = append(s.deletedResources[dr], filepath.Join(namespace, name))
				s.mu.Unlock()
				return obj.Object, nil
			}
		}
		return nil, trace.NotFound("teleportrole %q not found", filepath.Join(namespace, name))
	}
}

func (s *KubeMockServer) getCRD(crd *CRD) httplib.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
		namespace := p.ByName("namespace")
		name := p.ByName("name")
		filter := func(obj runtime.Object) bool {
			namer, ok1 := obj.(interface{ GetName() string })
			if !ok1 || namer.GetName() != name {
				return false
			}
			nser, ok2 := obj.(interface{ GetNamespace() string })
			return namespace == "" || (ok2 && nser.GetNamespace() == namespace)
		}
		for _, obj := range crd.items {
			if filter(obj.Object) {
				return obj.Object, nil
			}
		}
		return nil, trace.NotFound("teleport %q not found", filepath.Join(namespace, name))
	}
}

func (s *KubeMockServer) listCRDs(crd *CRD) httplib.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
		var items []runtime.RawExtension

		namespace := p.ByName("namespace")
		filter := func(obj runtime.Object) bool {
			if namespace == "" {
				return true
			}
			nser, ok := obj.(interface{ GetNamespace() string })
			if !ok {
				return false
			}
			return namespace == nser.GetNamespace()
		}
		for _, obj := range crd.items {
			if filter(obj.Object) {
				items = append(items, obj)
			}
		}
		return metav1.List{
			TypeMeta: metav1.TypeMeta{
				Kind:       crd.listKind,
				APIVersion: path.Join(crd.group, crd.version),
			},
			ListMeta: metav1.ListMeta{
				ResourceVersion: "1231415",
			},
			Items: items,
		}, nil
	}
}

func (s *KubeMockServer) DeletedCRDs(kind, reqID string) []string {
	s.mu.Lock()
	key := deletedResource{kind: kind, requestID: reqID}
	deleted := make([]string, len(s.deletedResources[key]))
	copy(deleted, s.deletedResources[key])
	s.mu.Unlock()
	sort.Strings(deleted)
	return deleted
}
