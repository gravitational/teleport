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

package version

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	v1 "k8s.io/api/core/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const authVersionKeyName = "agent-auth-version"

// AuthVersionGetter gets the auth server version.
type AuthVersionGetter interface {
	Get(context.Context, kclient.Object) (string, error)
}

type authVersionGetter struct {
	kclient.Client
}

// NewAuthVersionGetter creates a new AuthVersionGetter
func NewAuthVersionGetter(client kclient.Client) AuthVersionGetter {
	return &authVersionGetter{Client: client}
}

// Get returns the auth version stored in the shared state secret
func (a *authVersionGetter) Get(ctx context.Context, object kclient.Object) (string, error) {
	secretName := fmt.Sprintf("%s-shared-state", object.GetName())
	var secret v1.Secret
	err := a.Client.Get(ctx, kclient.ObjectKey{Namespace: object.GetNamespace(), Name: secretName}, &secret)
	if err != nil {
		return "", trace.Wrap(err)
	}
	rawData, ok := secret.Data[authVersionKeyName]
	if !ok {
		return "", trace.Errorf("secret %s does not have key %s", secretName, authVersionKeyName)
	}
	version, err := EnsureSemver(string(rawData))
	if err != nil {
		return "", trace.Wrap(err)
	}
	return version, nil
}
