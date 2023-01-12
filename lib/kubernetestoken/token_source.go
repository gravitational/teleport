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

package kubernetestoken

import (
	"strings"

	"github.com/gravitational/trace"
)

const kubernetesDefaultTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

type getEnvFunc func(key string) string
type readFileFunc func(name string) ([]byte, error)

func GetIDToken(getEnv getEnvFunc, readFile readFileFunc) (string, error) {
	// We check if we should use a custom location instead of the default one. This env var is not standard.
	// This is useful when the operator wants to use a custom projected token, or another service account.
	path := kubernetesDefaultTokenPath
	if customPath := getEnv("KUBERNETES_TOKEN_PATH"); customPath != "" {
		path = customPath
	}

	token, err := readFile(path)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}

	// Usually kubernetes tokens don't start or end with newlines, but better safe than sorry
	return strings.TrimSpace(string(token)), nil
}
