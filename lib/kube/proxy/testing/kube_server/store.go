/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/gravitational/teleport/lib/httplib"
)

type storedResource struct {
	group      string
	version    string
	resource   string
	kind       string
	listKind   string
	namespaced bool
}

type storedResourceKey struct {
	group     string
	version   string
	resource  string
	namespace string
	name      string
}

func (r storedResource) key(namespace, name string) storedResourceKey {
	if !r.namespaced {
		namespace = ""
	}

	return storedResourceKey{
		group:     r.group,
		version:   r.version,
		resource:  r.resource,
		namespace: namespace,
		name:      name,
	}
}

func (s *KubeMockServer) registerStoredResource(router *http.ServeMux, r storedResource) {
	base := storedResourcePath(r)
	router.Handle("POST "+base, s.withWriter(s.createStoredObject(r)))
	router.Handle("GET "+base, s.withWriter(s.listStoredObjects(r)))
	router.Handle("GET "+base+"/{name}", s.withWriter(s.getStoredObject(r)))
	router.Handle("PUT "+base+"/{name}", s.withWriter(s.updateStoredObject(r)))
	router.Handle("DELETE "+base+"/{name}", s.withWriter(s.deleteStoredObject(r)))
	if r.namespaced {
		router.Handle("GET "+storedResourceClusterPath(r), s.withWriter(s.listStoredObjects(r)))
	}
}

func storedResourcePath(r storedResource) string {
	if r.group == "" {
		if r.namespaced {
			return "/api/{ver}/namespaces/{namespace}/" + r.resource
		}

		return "/api/{ver}/" + r.resource
	}

	if r.namespaced {
		return "/apis/" + r.group + "/{ver}/namespaces/{namespace}/" + r.resource
	}

	return "/apis/" + r.group + "/{ver}/" + r.resource
}

func storedResourceClusterPath(r storedResource) string {
	if r.group == "" {
		return "/api/{ver}/" + r.resource
	}

	return "/apis/" + r.group + "/{ver}/" + r.resource
}

func (s *KubeMockServer) createStoredObject(r storedResource) httplib.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
		obj, err := parseUnstructured(req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if obj.GetName() == "" {
			return nil, trace.BadParameter("%s name is required", r.kind)
		}

		namespace := p.ByName("namespace")
		if r.namespaced {
			obj.SetNamespace(namespace)
		} else {
			obj.SetNamespace("")
		}

		obj.SetGroupVersionKind(schema.GroupVersionKind{Group: r.group, Version: r.version, Kind: r.kind})
		if obj.GetResourceVersion() == "" {
			obj.SetResourceVersion("1")
		}

		if obj.GetUID() == "" {
			obj.SetUID(types.UID(filepath.Join(r.resource, obj.GetNamespace(), obj.GetName())))
		}

		if obj.GetCreationTimestamp().Time.IsZero() {
			obj.SetCreationTimestamp(metav1.Now())
		}

		key := r.key(namespace, obj.GetName())
		s.mu.Lock()
		defer s.mu.Unlock()

		if _, ok := s.objects[key]; ok {
			return nil, trace.AlreadyExists("%s %q already exists", r.kind, filepath.Join(namespace, obj.GetName()))
		}
		s.objects[key] = obj.DeepCopy()

		return obj, nil
	}
}

func (s *KubeMockServer) getStoredObject(r storedResource) httplib.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
		key := r.key(p.ByName("namespace"), p.ByName("name"))
		s.mu.Lock()
		defer s.mu.Unlock()

		obj, ok := s.objects[key]
		if !ok {
			return nil, trace.NotFound("%s %q not found", r.kind, filepath.Join(key.namespace, key.name))
		}

		return obj.DeepCopy(), nil
	}
}

func (s *KubeMockServer) listStoredObjects(r storedResource) httplib.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
		selector, err := labels.Parse(req.URL.Query().Get("labelSelector"))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		namespace := p.ByName("namespace")
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(schema.GroupVersionKind{Group: r.group, Version: r.version, Kind: r.listKind})
		list.SetResourceVersion("1231415")

		s.mu.Lock()
		defer s.mu.Unlock()

		for key, obj := range s.objects {
			if key.group != r.group || key.version != r.version || key.resource != r.resource {
				continue
			}

			if r.namespaced && namespace != "" && key.namespace != namespace {
				continue
			}

			if !selector.Matches(labels.Set(obj.GetLabels())) {
				continue
			}

			list.Items = append(list.Items, *obj.DeepCopy())
		}

		slices.SortFunc(list.Items, func(a, b unstructured.Unstructured) int {
			return strings.Compare(filepath.Join(a.GetNamespace(), a.GetName()), filepath.Join(b.GetNamespace(), b.GetName()))
		})

		return list, nil
	}
}

func (s *KubeMockServer) updateStoredObject(r storedResource) httplib.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
		obj, err := parseUnstructured(req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		name := p.ByName("name")
		namespace := p.ByName("namespace")
		if r.namespaced {
			obj.SetNamespace(namespace)
		} else {
			obj.SetNamespace("")
		}
		obj.SetName(name)
		obj.SetGroupVersionKind(schema.GroupVersionKind{Group: r.group, Version: r.version, Kind: r.kind})

		key := r.key(namespace, name)
		s.mu.Lock()
		defer s.mu.Unlock()
		existing, ok := s.objects[key]
		if !ok {
			return nil, trace.NotFound("%s %q not found", r.kind, filepath.Join(key.namespace, key.name))
		}
		if obj.GetUID() == "" {
			obj.SetUID(existing.GetUID())
		}
		rv, _ := strconv.Atoi(existing.GetResourceVersion())
		obj.SetResourceVersion(strconv.Itoa(rv + 1))
		s.objects[key] = obj.DeepCopy()
		return obj, nil
	}
}

func (s *KubeMockServer) deleteStoredObject(r storedResource) httplib.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
		key := r.key(p.ByName("namespace"), p.ByName("name"))
		s.mu.Lock()
		defer s.mu.Unlock()
		obj, ok := s.objects[key]
		if !ok {
			return nil, trace.NotFound("%s %q not found", r.kind, filepath.Join(key.namespace, key.name))
		}

		delete(s.objects, key)
		return obj.DeepCopy(), nil
	}
}

func parseUnstructured(req *http.Request) (*unstructured.Unstructured, error) {
	data, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(data) == 0 {
		return nil, trace.BadParameter("no body")
	}

	obj, gvk, err := kubeCodecs.UniversalDeserializer().Decode(data, nil, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if u, ok := obj.(*unstructured.Unstructured); ok {
		return u, nil
	}

	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	u := &unstructured.Unstructured{Object: m}
	if gvk != nil {
		u.SetGroupVersionKind(*gvk)
	}

	return u, nil
}
