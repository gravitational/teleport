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

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gravitational/teleport/lib/backend"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/trace"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyconfigv1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	secretIdentifierName   = "state"
	namespaceEnv           = "KUBE_NAMESPACE"
	teleportReplicaNameEnv = "TELEPORT_REPLICA_NAME"
	releaseNameEnv         = "RELEASE_NAME"
)

// InKubeCluster detemines if the agent is running inside a Kubernetes cluster and has access to
// service account token and cluster CA. Besides, it also validates the presence of `KUBE_NAMESPACE`
// and `TELEPORT_REPLICA_NAME` environment variables to generate the secret name.
func InKubeCluster() bool {
	_, _, err := kubeutils.GetKubeClient("")

	return err == nil &&
		len(os.Getenv(namespaceEnv)) > 0 &&
		len(os.Getenv(teleportReplicaNameEnv)) > 0
}

// Backend uses Kubernetes Secrets to store identities.
type Backend struct {
	// kubernetes client
	k8sClientSet *kubernetes.Clientset
	namespace    string
	secretName   string
	replicaName  string

	// Mutex is used to limit the number of concurrent operations per agent to 1 so we do not need
	// to handle retries locally.
	// The same happens with SQlite backend.
	mu *sync.Mutex
}

// New returns a new instance of Kubernetes Secret identity backend storage.
func New() (*Backend, error) {

	restClient, _, err := kubeutils.GetKubeClient("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Backend{
		k8sClientSet: restClient,
		namespace:    os.Getenv(namespaceEnv),
		replicaName:  os.Getenv(teleportReplicaNameEnv),
		secretName: fmt.Sprintf(
			"%s-%s",
			os.Getenv(teleportReplicaNameEnv),
			secretIdentifierName,
		),
		mu: &sync.Mutex{},
	}, nil
}

// Exists checks if the secret already exists in Kubernetes.
// It's used to determine if the agent never created a secret and might upgrade from
// local SQLite database. In that case, the agent reads local database and
// creates a copy of the keys in Kube Secret.
func (b *Backend) Exists(ctx context.Context) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	_, err := b.getSecret(ctx)
	return err == nil
}

// Get reads the secret and extracts the key from it.
// If the secret does not exist or the key is not found it returns trace.Notfound,
// otherwise returns the underlying error.
func (b *Backend) Get(ctx context.Context, key []byte) (*backend.Item, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.readSecretData(ctx, key)
}

// Create creates item
func (b *Backend) Create(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.updateSecretContent(ctx, i)
}

// Put puts value into backend (creates if it does not exist, updates it otherwise)
func (b *Backend) Put(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.updateSecretContent(ctx, i)
}

// PutItems receives multiple items and upserts them into the Kubernetes Secret.
// This function is only used when the Agent's Secret does not exist, but local SQLite database
// has identity credentials.
// TODO(tigrato): remove this once the compatibility layer between local storage and
// Kube secret storage is no longer required!
func (b *Backend) PutItems(ctx context.Context, items ...backend.Item) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	_, err := b.updateSecretContent(ctx, items...)
	return err
}

// getSecret reads the secret from K8S API.
func (b *Backend) getSecret(ctx context.Context) (*corev1.Secret, error) {
	secret, err := b.k8sClientSet.
		CoreV1().
		Secrets(b.namespace).
		Get(ctx, b.secretName, metav1.GetOptions{})

	if err == nil {
		return secret, nil
	}
	kubeErr := &kubeerrors.StatusError{}
	if errors.As(err, &kubeErr) && kubeErr.ErrStatus.Code == http.StatusNotFound {
		return nil, trace.NotFound("secret %v not found", b.secretName)
	}
	return nil, trace.Wrap(err)
}

// readSecretData reads the secret content and extracts the content for key.
// returns an error if the key does not exist or the data is empty.
func (b *Backend) readSecretData(ctx context.Context, key []byte) (*backend.Item, error) {
	secret, err := b.getSecret(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	data, ok := secret.Data[backendKeyToSecret(key)]
	if !ok || len(data) == 0 {
		return nil, trace.NotFound("key [%s] not found in secret %s", string(key), b.secretName)
	}

	return &backend.Item{
		Key:   key,
		Value: data,
	}, nil
}

func (b *Backend) updateSecretContent(ctx context.Context, items ...backend.Item) (*backend.Lease, error) {
	// FIXME(tigrato):
	// for now, the agent is the owner of the secret so it's safe to replace changes

	secret, err := b.getSecret(ctx)
	if err != nil && trace.IsNotFound(err) {
		secret, err = b.createSecret(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}

	for _, item := range items {
		secret.Data[backendKeyToSecret(item.Key)] = item.Value
	}

	if err := b.updateSecret(ctx, secret); err != nil {
		return nil, trace.Wrap(err)
	}

	return &backend.Lease{}, nil
}

func (b *Backend) updateSecret(ctx context.Context, secret *corev1.Secret) error {
	secretApply := applyconfigv1.Secret(b.secretName, b.namespace).
		WithResourceVersion(secret.ResourceVersion).
		WithData(secret.Data).
		WithLabels(secret.GetLabels()).
		WithAnnotations(secret.GetAnnotations())

	_, err := b.k8sClientSet.
		CoreV1().
		Secrets(b.namespace).
		Apply(ctx, secretApply, metav1.ApplyOptions{FieldManager: b.replicaName})

	return trace.Wrap(err)
}

func (b *Backend) createSecret(ctx context.Context) (*corev1.Secret, error) {
	const (
		helmReleaseNameAnnotation     = "meta.helm.sh/release-name"
		helmReleaseNamesaceAnnotation = "meta.helm.sh/release-namespace"
		helmK8SManaged                = "app.kubernetes.io/managed-by"
		helmResourcePolicy            = "helm.sh/resource-policy"
	)
	secretApply := applyconfigv1.Secret(b.secretName, b.namespace).
		WithData(map[string][]byte{}).
		WithLabels(map[string]string{
			helmK8SManaged: "Helm",
		}).
		WithAnnotations(map[string]string{
			helmReleaseNameAnnotation:     os.Getenv(releaseNameEnv),
			helmReleaseNamesaceAnnotation: os.Getenv(namespaceEnv),
			helmResourcePolicy:            "keep",
		})

	return b.k8sClientSet.
		CoreV1().
		Secrets(b.namespace).
		Apply(
			ctx,
			secretApply,
			metav1.ApplyOptions{FieldManager: b.replicaName},
		)

}

// backendKeyToSecret replaces the "/" with "."
// "/" chars are not allowed in Kubernetes Secret keys.
func backendKeyToSecret(k []byte) string {
	return strings.ReplaceAll(string(k), "/", ".")
}
