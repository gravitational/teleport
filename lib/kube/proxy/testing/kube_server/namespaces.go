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
	"io"
	"net/http"
	"slices"
	"sort"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/kube/proxy/responsewriters"
)

var defaultNamespaceList = corev1.NamespaceList{
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

func (s *KubeMockServer) listNamespaces(
	w http.ResponseWriter,
	req *http.Request,
	p httprouter.Params,
) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	list := s.nsList.DeepCopy()
	return list, nil
}

func (s *KubeMockServer) getNamespace(
	w http.ResponseWriter,
	req *http.Request,
	p httprouter.Params,
) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := p.ByName("name")
	filter := func(ns corev1.Namespace) bool {
		return ns.Name == name
	}
	for _, ns := range s.nsList.Items {
		if filter(ns) {
			return ns, nil
		}
	}
	return nil, trace.NotFound("namespace %q not found", name)
}

func parseNamespace(req *http.Request) (*corev1.Namespace, error) {
	data, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(data) == 0 {
		return nil, trace.BadParameter("no body")
	}
	decoder, err := newDecoderForContentType(
		req.Header.Get(responsewriters.ContentTypeHeader),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	into := corev1.Namespace{}
	objI, _, err := decoder.Decode(data, nil /* defaults */, &into)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	obj, ok := objI.(*corev1.Namespace)
	if !ok {
		return nil, trace.BadParameter(
			"expected *corev1.Namespace, got %T", objI,
		)
	}
	return obj, nil
}

func (s *KubeMockServer) createNamespace(
	w http.ResponseWriter,
	req *http.Request,
	p httprouter.Params,
) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns, err := parseNamespace(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if ns.Name == "" {
		return nil, trace.BadParameter("namespace name is required")
	}

	if slices.ContainsFunc(s.nsList.Items, func(existing corev1.Namespace) bool {
		return existing.Name == ns.Name
	}) {
		return nil, trace.AlreadyExists(
			"namespace %q already exists", ns.Name,
		)
	}

	ns.ResourceVersion = "1"
	s.nsList.Items = append(s.nsList.Items, *ns)
	return &ns, nil
}

func (s *KubeMockServer) deleteNamespace(
	w http.ResponseWriter,
	req *http.Request,
	p httprouter.Params,
) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	name := p.ByName("name")

	deleteOpts, err := parseDeleteCollectionBody(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reqID := ""
	if deleteOpts.Preconditions != nil && deleteOpts.Preconditions.UID != nil {
		reqID = string(*deleteOpts.Preconditions.UID)
	}
	filter := func(ns corev1.Namespace) bool {
		return ns.Name == name
	}
	for i, ns := range s.nsList.Items {
		if filter(ns) {
			key := deletedResource{reqID, types.KindKubeNamespace}
			s.deletedResources[key] = append(s.deletedResources[key], name)
			s.nsList.Items = slices.Delete(s.nsList.Items, i, i+1)
			return ns, nil
		}
	}
	return nil, trace.NotFound("namespace %q not found", name)
}

func (s *KubeMockServer) ListNamespaces() *corev1.NamespaceList {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nsList.DeepCopy()
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
