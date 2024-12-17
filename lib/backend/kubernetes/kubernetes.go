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

package kubernetes

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyconfigv1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/gravitational/teleport/lib/backend"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
)

const (
	// secretIdentifierName is the suffix used to construct the per-agent store.
	secretIdentifierName = "state"
	// sharedSecretIdentifierName is the suffix used to construct the shared store.
	sharedSecretIdentifierName = "shared-state"
	// NamespaceEnv is the env variable defined by the Helm chart that contains the
	// namespace value.
	NamespaceEnv = "KUBE_NAMESPACE"
	// ReleaseNameEnv is the env variable defined by the Helm chart that contains the
	// release name value.
	ReleaseNameEnv         = "RELEASE_NAME"
	teleportReplicaNameEnv = "TELEPORT_REPLICA_NAME"
)

// InKubeCluster detemines if the agent is running inside a Kubernetes cluster and has access to
// service account token and cluster CA. Besides, it also validates the presence of `KUBE_NAMESPACE`
// and `TELEPORT_REPLICA_NAME` environment variables to generate the secret name.
func InKubeCluster() bool {
	_, _, err := kubeutils.GetKubeClient("")

	return err == nil &&
		len(os.Getenv(NamespaceEnv)) > 0 &&
		len(os.Getenv(teleportReplicaNameEnv)) > 0
}

// Config structure represents configuration section
type Config struct {
	// Namespace is the Agent's namespace
	// Field is required
	Namespace string
	// SecretName is the name of the kubernetes secret resource that backs this store. Conventionally
	// this will be set to '<replica-name>-state' for per-agent secret store, and '<release-name>-shared-state'
	// for the shared release-level store.
	// Field is required
	SecretName string
	// FieldManager is the name used to identify the "owner" of fields within
	// the store. This is the replica name in the per-agent state store, and
	// helm release name (or 'teleport') in the shared store.
	// Field is required.
	FieldManager string
	// ReleaseName is the HELM release name
	// Field is optional
	ReleaseName string
	// KubeClient is the Kubernetes rest client
	// Field is required
	KubeClient kubernetes.Interface
}

func (c Config) Check() error {
	if len(c.Namespace) == 0 {
		return trace.BadParameter("missing namespace")
	}

	if len(c.SecretName) == 0 {
		return trace.BadParameter("missing secret name")
	}

	if len(c.FieldManager) == 0 {
		return trace.BadParameter("missing field manager")
	}

	if c.KubeClient == nil {
		return trace.BadParameter("missing Kubernetes client")
	}

	return nil
}

// Backend implements a subset of the teleport backend API backed by a kuberentes secret resource
// and storing backend items as entries in the secret's 'data' map.
type Backend struct {
	Config

	// Mutex is used to limit the number of concurrent operations per agent to 1 so we do not need
	// to handle retries locally.
	// The same happens with SQlite backend.
	mu sync.Mutex
}

// New returns a new instance of Kubernetes Secret identity backend storage.
func New() (*Backend, error) {
	restClient, _, err := kubeutils.GetKubeClient("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewWithClient(restClient)
}

// NewWithClient returns a new instance of Kubernetes Secret identity backend storage with the provided client.
func NewWithClient(restClient kubernetes.Interface) (*Backend, error) {
	for _, env := range []string{teleportReplicaNameEnv, NamespaceEnv} {
		if len(os.Getenv(env)) == 0 {
			return nil, trace.BadParameter("environment variable %q not set or empty", env)
		}
	}

	return NewWithConfig(
		Config{
			Namespace: os.Getenv(NamespaceEnv),
			SecretName: fmt.Sprintf(
				"%s-%s",
				os.Getenv(teleportReplicaNameEnv),
				secretIdentifierName,
			),
			FieldManager: os.Getenv(teleportReplicaNameEnv),
			ReleaseName:  os.Getenv(ReleaseNameEnv),
			KubeClient:   restClient,
		},
	)
}

// NewShared returns a new instance of the kuberentes shared secret store (equivalent to New() except that
// this backend can be written to by any teleport agent within the helm release. used for propagating relevant state
// to controllers).
func NewShared() (*Backend, error) {
	restClient, _, err := kubeutils.GetKubeClient("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return NewSharedWithClient(restClient)
}

// NewSharedWithClient returns a new instance of the shared kubernetes secret store with the provided client (equivalent
// to NewWithClient() except that this backend can be written to by any teleport agent within the helm release. used for propagating
// relevant state to controllers).
func NewSharedWithClient(restClient kubernetes.Interface) (*Backend, error) {
	if os.Getenv(NamespaceEnv) == "" {
		return nil, trace.BadParameter("environment variable %q not set or empty", NamespaceEnv)
	}

	ident := os.Getenv(ReleaseNameEnv)
	if ident == "" {
		ident = "teleport"
		slog.WarnContext(context.Background(), "Var RELEASE_NAME is not set, falling back to default identifier teleport for shared store.")
	}

	return NewWithConfig(
		Config{
			Namespace: os.Getenv(NamespaceEnv),
			SecretName: fmt.Sprintf(
				"%s-%s",
				ident,
				sharedSecretIdentifierName,
			),
			FieldManager: ident,
			ReleaseName:  os.Getenv(ReleaseNameEnv),
			KubeClient:   restClient,
		},
	)
}

// NewWithConfig returns a new instance of Kubernetes Secret identity backend storage with the provided config.
func NewWithConfig(conf Config) (*Backend, error) {
	if err := conf.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Backend{
		Config: conf,
		mu:     sync.Mutex{},
	}, nil
}

func (b *Backend) GetName() string {
	return "kubernetes"
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
func (b *Backend) Get(ctx context.Context, key backend.Key) (*backend.Item, error) {
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

// getSecret reads the secret from K8S API.
func (b *Backend) getSecret(ctx context.Context) (*corev1.Secret, error) {
	secret, err := b.KubeClient.
		CoreV1().
		Secrets(b.Namespace).
		Get(ctx, b.SecretName, metav1.GetOptions{})

	if kubeerrors.IsNotFound(err) {
		return nil, trace.NotFound("secret %v not found", b.SecretName)
	}

	return secret, trace.Wrap(err)
}

// readSecretData reads the secret content and extracts the content for key.
// returns an error if the key does not exist or the data is empty.
func (b *Backend) readSecretData(ctx context.Context, key backend.Key) (*backend.Item, error) {
	secret, err := b.getSecret(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	data, ok := secret.Data[backendKeyToSecret(key)]
	if !ok || len(data) == 0 {
		return nil, trace.NotFound("key [%s] not found in secret %s", key.String(), b.SecretName)
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
	if trace.IsNotFound(err) {
		secret = b.genSecretObject()
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}

	updateDataMap(secret.Data, items...)

	if err := b.upsertSecret(ctx, secret); err != nil {
		return nil, trace.Wrap(err)
	}

	return &backend.Lease{}, nil
}

func (b *Backend) upsertSecret(ctx context.Context, secret *corev1.Secret) error {
	secretApply := applyconfigv1.Secret(b.SecretName, b.Namespace).
		WithData(secret.Data).
		WithLabels(secret.GetLabels()).
		WithAnnotations(secret.GetAnnotations())

	// apply resource lock if it's not a creation
	if len(secret.ResourceVersion) > 0 {
		secretApply = secretApply.WithResourceVersion(secret.ResourceVersion)
	}

	_, err := b.KubeClient.
		CoreV1().
		Secrets(b.Namespace).
		Apply(ctx, secretApply, metav1.ApplyOptions{FieldManager: b.FieldManager})

	return trace.Wrap(err)
}

func (b *Backend) genSecretObject() *corev1.Secret {
	return &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:        b.SecretName,
			Namespace:   b.Namespace,
			Annotations: generateSecretAnnotations(b.Namespace, b.ReleaseName),
		},
		Data: map[string][]byte{},
	}
}

func generateSecretAnnotations(namespace, releaseNameEnv string) map[string]string {
	const (
		helmReleaseNameAnnotation     = "meta.helm.sh/release-name"
		helmReleaseNamesaceAnnotation = "meta.helm.sh/release-namespace"
		helmResourcePolicy            = "helm.sh/resource-policy"
	)

	if len(releaseNameEnv) > 0 {
		return map[string]string{
			helmReleaseNameAnnotation:     releaseNameEnv,
			helmReleaseNamesaceAnnotation: namespace,
			helmResourcePolicy:            "keep",
		}
	}

	return map[string]string{}
}

// backendKeyToSecret replaces the "/" with "."
// "/" chars are not allowed in Kubernetes Secret keys.
func backendKeyToSecret(k backend.Key) string {
	return strings.ReplaceAll(k.String(), string(backend.Separator), ".")
}

func updateDataMap(data map[string][]byte, items ...backend.Item) {
	for _, item := range items {
		data[backendKeyToSecret(item.Key)] = item.Value
	}
}
