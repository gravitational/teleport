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

package main

import (
	"os"
	"strings"

	"github.com/gravitational/trace"
)

const (
	namespacePath   = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	namespaceEnvVar = "POD_NAMESPACE"
)

func GetKubernetesNamespace() (string, error) {
	namespace := os.Getenv(namespaceEnvVar)
	if namespace != "" {
		return namespace, nil
	}

	bs, err := os.ReadFile(namespacePath)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return strings.TrimSpace(string(bs)), nil
}
