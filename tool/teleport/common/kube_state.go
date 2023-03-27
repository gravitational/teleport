// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"github.com/gravitational/trace"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"

	kubestorage "github.com/gravitational/teleport/lib/backend/kubernetes"
)

// onKubeStateDelete lists the Kubernetes Secrets in the same namespace it's running
// and deletes the secrets that follow this patten: {release_name}-{replica}-state.
func onKubeStateDelete() error {
	ctx := context.Background()
	namespace := os.Getenv(kubestorage.NamespaceEnv)
	if len(namespace) == 0 {
		return trace.BadParameter("invalid namespace provided")
	}
	releaseName := os.Getenv(kubestorage.ReleaseNameEnv)
	if len(namespace) == 0 {
		return trace.BadParameter("invalid release name provided")
	}
	secretRegex, err := regexp.Compile(fmt.Sprintf(`%s-[0-9]+-state`, releaseName))
	if err != nil {
		return trace.Wrap(err)
	}
	// This command is run when the user uninstalls the teleport-kube-agent, which
	// means we are running on a Kubernetes cluster.
	config, err := restclient.InClusterConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return trace.Wrap(err)
	}

	// List the secrets available in the cluster.
	rsp, err := clientset.CoreV1().Secrets(namespace).List(ctx, v1.ListOptions{})
	if err != nil {
		return trace.Wrap(err)
	}
	var errs []error
	for _, secret := range rsp.Items {
		if !secretRegex.MatchString(secret.Name) {
			// Secret name is not a kube state secret.
			// Format: {.Release.Name}-{replica}-state
			continue
		}
		// Deletes every secret that matches
		if err := clientset.CoreV1().Secrets(namespace).Delete(
			ctx,
			secret.Name,
			v1.DeleteOptions{},
		); err != nil {
			errs = append(errs, err)
		}
	}
	return trace.NewAggregate(errs...)
}
