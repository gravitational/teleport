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

package crd

import (
	"context"
	"embed"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	resourcesv2 "github.com/gravitational/teleport/operator/apis/resources/v2"
	resourcesv5 "github.com/gravitational/teleport/operator/apis/resources/v5"
	//+kubebuilder:scaffold:imports
)

func TestUserCreation(t *testing.T) {
	ctx := context.Background()
	k8sClient := startKubernetesOperator(t)
	crdFS := embed.FS{}
	Upsert(ctx, logr.Logger{}, crdFS, k8sClient)

}

func startKubernetesOperator(t *testing.T) kclient.Client {
	testEnv := &envtest.Environment{}

	cfg, err := testEnv.Start()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	err = resourcesv5.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	err = resourcesv2.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	k8sClient, err := kclient.New(cfg, kclient.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)
	require.NotNil(t, k8sClient)

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	require.NoError(t, err)

	ctx, ctxCancel := context.WithCancel(context.Background())
	go func() {
		err = k8sManager.Start(ctx)
		require.NoError(t, err)
	}()

	t.Cleanup(func() {
		ctxCancel()
		err = testEnv.Stop()
		require.NoError(t, err)
	})

	return k8sClient
}
