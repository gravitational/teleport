/*
Copyright 2022 Gravitational, Inc.

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
	deleteOpts, err := parseDeleteCollectionBody(req.Body)
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
