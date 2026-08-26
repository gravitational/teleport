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
