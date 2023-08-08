/*
Copyright 2023 Gravitational, Inc.

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

package kubernetestoken

import (
	"context"
	"os"
	"time"

	"github.com/gravitational/trace"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const podNamespaceEnv = "POD_NAMESPACE"
const kubeConfigEnv = "KUBECONFIG"

type RequestTokenOpts struct {
	// Audience is the audience to request be placed inside the `aud` claim
	// of the returned JWT.
	Audience string
	// ServiceAccount is the name of the service account to request a JWT for.
	ServiceAccount string
	// ServiceAccountNamespace is the namespace of the service account to
	// request a JWT for.
	// If unspecified, this falls back to the value in POD_NAMESPACE.
	ServiceAccountNamespace string

	// TTL is the amount of time the token should be valid for from issue.
	// This will be truncated to the nearest second.
	TTL time.Duration

	// TODO: Support overriding the path to the service account to use to
	// request the token.

	getEnvFunc getEnvFunc
}

func (o *RequestTokenOpts) checkAndSetDefaults() error {
	if o.getEnvFunc == nil {
		o.getEnvFunc = os.Getenv
	}

	switch {
	case o.Audience == "":
		return trace.BadParameter("audience must be specified")
	case o.ServiceAccount == "":
		return trace.BadParameter("service account must be specified")
	case o.TTL == time.Duration(0):
		return trace.BadParameter("ttl must be specified")

	}

	if o.ServiceAccountNamespace == "" {
		// First try to determine pod namespace from POD_NAMESPACE
		o.ServiceAccountNamespace = o.getEnvFunc(podNamespaceEnv)

		if o.ServiceAccountNamespace == "" {
			return trace.BadParameter("")
		}
	}

	return nil
}

func RequestToken(ctx context.Context, opts RequestTokenOpts) (string, error) {
	if err := opts.checkAndSetDefaults(); err != nil {
		return "", trace.Wrap(err)
	}

	// BuildConfigFromFlags falls back to InClusterConfig if both params
	// are empty. This means KUBECONFIG takes precedence.
	clientCfg, err := clientcmd.BuildConfigFromFlags(
		"",
		opts.getEnvFunc(kubeConfigEnv),
	)
	if err != nil {
		return "", trace.Wrap(err)
	}
	k8s, err := kubernetes.NewForConfig(clientCfg)
	if err != nil {
		return "", trace.Wrap(err)
	}

	// Truncate the TTL duration down to seconds as required by the k8s api.
	expirationInSeconds := int64(opts.TTL.Seconds())

	resp, err := k8s.CoreV1().ServiceAccounts(opts.ServiceAccountNamespace).CreateToken(
		ctx,
		opts.ServiceAccount,
		&authv1.TokenRequest{
			Spec: authv1.TokenRequestSpec{
				Audiences:         []string{opts.Audience},
				ExpirationSeconds: &expirationInSeconds,
				// We don't bind to the object as this will fail when requesting
				// a token for an SA that isn't the one associated with the
				// pod.
				BoundObjectRef: nil,
			},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return resp.Status.Token, nil
}
