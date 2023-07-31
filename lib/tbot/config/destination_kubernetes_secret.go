package config

import (
	"context"
	"fmt"
	"github.com/gravitational/trace"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyconfigv1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"sync"
)

const DestinationKubernetesSecretType = "kubernetes_secret"
const kubernetesNamespaceEnv = "POD_NAMESPACE"

type DestinationKubernetesSecret struct {
	// Name is the name the Kubernetes Secret that should be created and written
	// to.
	Name string `yaml:"name"`

	mu        sync.Mutex
	namespace string
	k8s       kubernetes.Interface
}

func (dks *DestinationKubernetesSecret) fieldManager() string {
	return "tbot"
}

func (dks *DestinationKubernetesSecret) getSecret(ctx context.Context) (*corev1.Secret, error) {
	secret, err := dks.k8s.CoreV1().Secrets(dks.namespace).Get(ctx, dks.Name, v1.GetOptions{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// This will be nil out of Kubernetes if it hasn't had any values provided.
	// Replace with an initialized map so code using this function does not
	// need to worry about writing/reading from a nil map.
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	return secret, nil
}

func (dks *DestinationKubernetesSecret) secretTemplate() *corev1.Secret {
	return &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: v1.ObjectMeta{
			Name:      dks.Name,
			Namespace: dks.namespace,
		},
		Data: map[string][]byte{},
	}
}

func (dks *DestinationKubernetesSecret) upsertSecret(ctx context.Context, secret *corev1.Secret) error {
	apply := applyconfigv1.Secret(dks.Name, dks.namespace).
		WithData(secret.Data).
		WithResourceVersion(secret.ResourceVersion).
		WithType(secret.Type)

	_, err := dks.k8s.CoreV1().Secrets(dks.namespace).Apply(ctx, apply, v1.ApplyOptions{
		FieldManager: dks.fieldManager(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (dks *DestinationKubernetesSecret) Verify(_ []string) error {
	return nil
}

func (dks *DestinationKubernetesSecret) TryLock() (func() error, error) {
	// No locking support currently implemented. Users will need to be cautious
	// to not point two tbots to the same secret.
	return func() error { return nil }, nil
}

func (dks *DestinationKubernetesSecret) CheckAndSetDefaults() error {
	if dks.Name == "" {
		return trace.BadParameter("name must not be empty")
	}

	return nil
}

func (dks *DestinationKubernetesSecret) Init(ctx context.Context, subdirs []string) error {
	dks.mu.Lock()
	defer dks.mu.Unlock()

	if dks.namespace == "" {
		dks.namespace = os.Getenv(kubernetesNamespaceEnv)
		if dks.namespace == "" {
			return trace.BadParameter("unable to detect namespace from %s environment variable", kubernetesNamespaceEnv)
		}
	}

	if len(subdirs) > 0 {
		return trace.BadParameter("kubernetes_secret destination does not support subdirectories")
	}

	// If no k8s client is injected, we attempt to create one from the
	// environment.
	if dks.k8s == nil {
		// BuildConfigFromFlags falls back to InClusterConfig if both params
		// are empty. This means KUBECONFIG takes precedence.
		clientCfg, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		if err != nil {
			return trace.Wrap(err)
		}
		dks.k8s, err = kubernetes.NewForConfig(clientCfg)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Try and read the secret and then write to it. This lets us fail early
	// and clearly if the appropriate permissions are missing
	secret, err := dks.getSecret(ctx)
	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return trace.Wrap(err)
		}
		secret = dks.secretTemplate()
	}
	if err := dks.upsertSecret(ctx, secret); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (dks *DestinationKubernetesSecret) Write(ctx context.Context, name string, data []byte) error {
	dks.mu.Lock()
	defer dks.mu.Unlock()

	secret, err := dks.getSecret(ctx)
	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return trace.Wrap(err)
		}
		log.Warn("Kubernetes secret missing on attempt to write data- will create.")
		// If the secret doesn't exist, we create it on write - this is ensures
		// that we can recover if the secret is deleted between renewal loops.
		secret = dks.secretTemplate()
	}

	secret.Data[name] = data

	err = dks.upsertSecret(ctx, secret)
	return trace.Wrap(err)
}

func (dks *DestinationKubernetesSecret) Read(ctx context.Context, name string) ([]byte, error) {
	dks.mu.Lock()
	defer dks.mu.Unlock()

	secret, err := dks.getSecret(ctx)
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return nil, trace.NotFound("secret could not be found")
		}
		return nil, trace.Wrap(err)
	}

	data, ok := secret.Data[name]
	if !ok {
		return nil, trace.NotFound("key %q cannot be found in secret data", name)
	}

	return data, nil
}

func (dks *DestinationKubernetesSecret) String() string {
	return fmt.Sprintf("%s: %s", DestinationKubernetesSecretType, dks.Name)
}

func (dks DestinationKubernetesSecret) MarshalYAML() (interface{}, error) {
	type raw DestinationKubernetesSecret
	return withTypeHeader(raw(dks), DestinationKubernetesSecretType)
}
