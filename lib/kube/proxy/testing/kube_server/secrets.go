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

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/teleport/api/types"
)

var secretList = corev1.SecretList{
	TypeMeta: metav1.TypeMeta{
		Kind:       "SecretList",
		APIVersion: "v1",
	},
	ListMeta: metav1.ListMeta{
		ResourceVersion: "1231415",
	},
	Items: []corev1.Secret{
		newSecret("secret-1", "default"),
		newSecret("secret-2", "default"),
		newSecret("test", "default"),
		newSecret("secret-1", "dev"),
		newSecret("secret-2", "dev"),
	},
}

func newSecret(name, namespace string) corev1.Secret {
	return corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func (s *KubeMockServer) listSecrets(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	items := []corev1.Secret{}

	namespace := p.ByName("namespace")
	filter := func(secret corev1.Secret) bool {
		return len(namespace) == 0 || namespace == secret.Namespace
	}
	for _, secret := range secretList.Items {
		if filter(secret) {
			items = append(items, secret)
		}
	}
	return &corev1.SecretList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SecretList",
			APIVersion: "v1",
		},
		ListMeta: metav1.ListMeta{
			ResourceVersion: "1231415",
		},
		Items: items,
	}, nil
}

func (s *KubeMockServer) getSecret(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
	namespace := p.ByName("namespace")
	name := p.ByName("name")
	filter := func(secret corev1.Secret) bool {
		return secret.Name == name && namespace == secret.Namespace
	}
	for _, secret := range secretList.Items {
		if filter(secret) {
			return secret, nil
		}
	}
	return nil, trace.NotFound("secret %q not found", filepath.Join(namespace, name))
}

func (s *KubeMockServer) deleteSecret(w http.ResponseWriter, req *http.Request, p httprouter.Params) (any, error) {
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
	filter := func(secret corev1.Secret) bool {
		return secret.Name == name && namespace == secret.Namespace
	}
	for _, secret := range secretList.Items {
		if filter(secret) {
			s.mu.Lock()
			s.deletedResources[deletedResource{kind: types.KindKubeSecret, requestID: reqID}] = append(s.deletedResources[deletedResource{kind: types.KindKubeSecret, requestID: reqID}], filepath.Join(namespace, name))
			s.mu.Unlock()
			return secret, nil
		}
	}
	return nil, trace.NotFound("secret %q not found", filepath.Join(namespace, name))
}
