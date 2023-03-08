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

package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"
	appsv1 "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corescheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	runtimescheme "sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	SchemeBuilder = &runtimescheme.Builder{GroupVersion: appsv1.SchemeGroupVersion}
	scheme        = runtime.NewScheme()
)

func init() {
	SchemeBuilder.Register(
		&appsv1.Deployment{},
		&appsv1.DeploymentList{},
		&appsv1.StatefulSet{},
		&appsv1.StatefulSetList{},
	)
	utilruntime.Must(SchemeBuilder.AddToScheme(scheme))
}

func setupTest(t *testing.T) (manager.Manager, kclient.Client, string) {
	testEnv := &envtest.Environment{}
	cfg, err := testEnv.Start()

	require.NoError(t, err)
	require.NotNil(t, cfg)

	namespace := validRandomResourceName("ns-")

	k8sClient, err := kclient.New(cfg, kclient.Options{Scheme: corescheme.Scheme})
	require.NoError(t, err)
	ns := &core.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	}

	ctx := context.Background()

	err = k8sClient.Create(ctx, ns)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = testEnv.Stop()
		require.NoError(t, err)
	})

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
	})
	require.NoError(t, err)

	return mgr, k8sClient, namespace

}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz1234567890")

func validRandomResourceName(prefix string) string {
	b := make([]rune, 5)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return prefix + string(b)
}
