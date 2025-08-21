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

package k8s

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applyconfigv1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
)

const SecretDestinationType = "kubernetes_secret"
const kubernetesNamespaceEnv = "POD_NAMESPACE"

type SecretDestination struct {
	// Name is the name the Kubernetes Secret that should be created and written
	// to.
	Name string `yaml:"name"`
	// Labels is a set of labels to apply to the output Kubernetes secret.
	// When configured, these labels will overwrite any existing labels on the
	// secret.
	Labels map[string]string `yaml:"labels,omitempty"`
	// Namespace to write the Kubernetes Secret to. If not specified, it
	// defaults to the value of the POD_NAMESPACE environment variable.
	//
	// When using the Helm chart, you'll need to additionally grant the tbot
	// service account permissions to read/write to the other namespace.
	Namespace string `yaml:"namespace,omitempty"`

	mu          sync.Mutex
	k8s         kubernetes.Interface
	initialized bool
}

func (dks *SecretDestination) getSecret(ctx context.Context) (*corev1.Secret, error) {
	secret, err := dks.k8s.CoreV1().Secrets(dks.Namespace).Get(ctx, dks.Name, v1.GetOptions{})
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

func (dks *SecretDestination) secretTemplate() *corev1.Secret {
	return &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: v1.ObjectMeta{
			Name:      dks.Name,
			Namespace: dks.Namespace,
			Labels:    dks.Labels,
		},
		Data: map[string][]byte{},
	}
}

func (dks *SecretDestination) upsertSecret(ctx context.Context, secret *corev1.Secret, dryRun bool) error {
	apply := applyconfigv1.Secret(dks.Name, dks.Namespace).
		WithData(secret.Data).
		WithResourceVersion(secret.ResourceVersion).
		WithType(secret.Type)

	// If user has configured labels, we overwrite the labels on the secret.
	if len(dks.Labels) > 0 {
		apply = apply.
			WithLabels(dks.Labels)
	}

	applyOpts := v1.ApplyOptions{
		FieldManager: "tbot",
	}
	if dryRun {
		applyOpts.DryRun = []string{"All"}
	}

	_, err := dks.k8s.CoreV1().Secrets(dks.Namespace).Apply(ctx, apply, applyOpts)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (dks *SecretDestination) Verify(_ []string) error {
	return nil
}

func (dks *SecretDestination) TryLock() (func() error, error) {
	// No locking support currently implemented. Users will need to be cautious
	// to not point two tbots to the same secret.
	return func() error { return nil }, nil
}

func (dks *SecretDestination) CheckAndSetDefaults() error {
	if dks.Name == "" {
		return trace.BadParameter("name must not be empty")
	}

	return nil
}

func (dks *SecretDestination) Init(ctx context.Context, subdirs []string) error {
	dks.mu.Lock()
	defer dks.mu.Unlock()
	if dks.initialized == true {
		return trace.BadParameter("destination has already been initialized")
	}

	if dks.Namespace == "" {
		log.DebugContext(
			ctx,
			"No explicit namespace provided for Kubernetes secret destination, attempting to detect from environment",
		)
		dks.Namespace = os.Getenv(kubernetesNamespaceEnv)
		if dks.Namespace == "" {
			return trace.BadParameter("unable to detect namespace from %s environment variable", kubernetesNamespaceEnv)
		}
	}

	if len(subdirs) > 0 {
		return trace.BadParameter("kubernetes_secret destination does not support subdirectories")
	}

	// If no k8s client is injected, we attempt to create one from the
	// environment.
	if dks.k8s == nil {
		var err error
		if dks.k8s, err = newKubernetesClient(); err != nil {
			return trace.Wrap(err)
		}
	}

	// Perform an initial dry-run attempt of applying the secret. This ensures
	// that we have the appropriate RBAC before proceeding, but avoids creating
	// a secret which will remain empty if something goes wrong later in the
	// credential generation.
	secret, err := dks.getSecret(ctx)
	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return trace.Wrap(err)
		}
		secret = dks.secretTemplate()
	}
	if err := dks.upsertSecret(ctx, secret, true); err != nil {
		return trace.Wrap(err)
	}

	dks.initialized = true
	return nil
}

func (dks *SecretDestination) Write(ctx context.Context, name string, data []byte) error {
	ctx, span := tracer.Start(
		ctx,
		"SecretDestination/Write",
		oteltrace.WithAttributes(attribute.String("name", name)),
	)
	defer span.End()

	dks.mu.Lock()
	defer dks.mu.Unlock()
	if dks.initialized == false {
		return trace.BadParameter("destination has not been initialized")
	}

	secret, err := dks.getSecret(ctx)
	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return trace.Wrap(err)
		}
		log.WarnContext(
			ctx,
			"Kubernetes secret missing on attempt to write data. One will be created.",
			"secret_name", dks.Name,
			"secret_namespace", dks.Namespace,
		)
		// If the secret doesn't exist, we create it on write - this is ensures
		// that we can recover if the secret is deleted between renewal loops.
		secret = dks.secretTemplate()
	}

	secret.Data[name] = data

	err = dks.upsertSecret(ctx, secret, false)
	return trace.Wrap(err)
}

// WriteMany allows you to write multiple artifacts to a destination at once.
// This should be atomic, meaning all artifacts are written or none are. Any
// artifacts that are not specified will be removed from the destination.
func (dks *SecretDestination) WriteMany(ctx context.Context, toWrite map[string][]byte) error {
	ctx, span := tracer.Start(
		ctx,
		"SecretDestination/WriteMany",
	)
	defer span.End()

	dks.mu.Lock()
	defer dks.mu.Unlock()
	if dks.initialized == false {
		return trace.BadParameter("destination has not been initialized")
	}

	secret, err := dks.getSecret(ctx)
	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return trace.Wrap(err)
		}
		log.WarnContext(
			ctx,
			"Kubernetes secret missing on attempt to write data. One will be created.",
			"secret_name", dks.Name,
			"secret_namespace", dks.Namespace,
		)
		// If the secret doesn't exist, we create it on write - this is ensures
		// that we can recover if the secret is deleted between renewal loops.
		secret = dks.secretTemplate()
	}

	secret.Data = toWrite

	err = dks.upsertSecret(ctx, secret, false)
	return trace.Wrap(err)
}

func (dks *SecretDestination) Read(ctx context.Context, name string) ([]byte, error) {
	ctx, span := tracer.Start(
		ctx,
		"SecretDestination/Read",
		oteltrace.WithAttributes(attribute.String("name", name)),
	)
	defer span.End()

	dks.mu.Lock()
	defer dks.mu.Unlock()
	if dks.initialized == false {
		return nil, trace.BadParameter("destination has not been initialized")
	}

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

func (dks *SecretDestination) String() string {
	return fmt.Sprintf(
		"%s: %s/%s",
		SecretDestinationType,
		dks.Namespace,
		dks.Name,
	)
}

func (dks *SecretDestination) MarshalYAML() (any, error) {
	type raw SecretDestination
	return encoding.WithTypeHeader((*raw)(dks), SecretDestinationType)
}

func (dks *SecretDestination) IsPersistent() bool {
	return true
}

func newKubernetesClient() (*kubernetes.Clientset, error) {
	// BuildConfigFromFlags falls back to InClusterConfig if both params
	// are empty. This means KUBECONFIG takes precedence.
	clientCfg, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	k8s, err := kubernetes.NewForConfig(clientCfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return k8s, nil
}
