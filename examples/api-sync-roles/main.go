// Copyright 2023 Gravitational, Inc
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

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
)

const (
	proxyAddr         string = ""
	initTimeout              = time.Duration(30) * time.Second
	identityFilePath  string = "auth.pem"
	kubeconfigPath    string = "kubeconfig"
	clusterName       string = "minikube"
	roleAnnotationKey string = "create-teleport-role"
)

func getRBACClient() (v1.RbacV1Interface, error) {
	f, err := os.Open(kubeconfigPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	kc, err := io.ReadAll(f)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	n, err := clientcmd.RESTConfigFromKubeConfig(kc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c, err := kubernetes.NewForConfig(n)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return c.RbacV1(), nil
}

func createTeleportRoleFromClusterRoleBinding(teleport *client.Client, k types.KubeCluster, r rbacv1.ClusterRoleBinding) error {
	if e, ok := r.Annotations[roleAnnotationKey]; !ok || e != "true" {
		return nil
	}

	role := types.RoleV6{}
	role.SetMetadata(types.Metadata{
		Name: k.GetName() + "-" + r.RoleRef.Name + "-" + "cluster",
	})

	b := k.GetStaticLabels()
	labels := make(types.Labels)
	for k, v := range b {
		labels[k] = []string{v}
	}
	role.SetKubernetesLabels(types.Allow, labels)
	role.SetKubeResources(types.Allow, []types.KubernetesResource{
		types.KubernetesResource{
			Kind:      "pod",
			Namespace: "*",
			Name:      "*",
		},
	})

	var g []string
	var u []string
	for _, s := range r.Subjects {
		if s.Kind == "User" || s.Kind == "ServiceAccount" {
			u = append(u, s.Name)
			continue
		}
		if s.Kind == "Group" {
			g = append(g, s.Name)
			continue
		}
	}
	role.SetKubeGroups(types.Allow, g)
	role.SetKubeUsers(types.Allow, u)
	if _, err := teleport.UpsertRole(
		context.Background(),
		&role,
	); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("Upserted Teleport role:", role.GetName())
	return nil
}

func createTeleportRoleFromRoleBinding(teleport *client.Client, k types.KubeCluster, r rbacv1.RoleBinding) error {
	if e, ok := r.Annotations[roleAnnotationKey]; !ok || e != "true" {
		return nil
	}

	role := types.RoleV6{}
	role.SetMetadata(types.Metadata{
		Name: k.GetName() + "-" + r.RoleRef.Name + "-" + r.Namespace,
	})

	b := k.GetStaticLabels()
	labels := make(types.Labels)
	for k, v := range b {
		labels[k] = []string{v}
	}
	role.SetKubernetesLabels(types.Allow, labels)
	role.SetKubeResources(types.Allow, []types.KubernetesResource{
		types.KubernetesResource{
			Kind:      "pod",
			Namespace: r.Namespace,
			Name:      "*",
		},
	})
	var g []string
	var u []string
	for _, s := range r.Subjects {
		if s.Kind == "User" || s.Kind == "ServiceAccount" {
			u = append(u, s.Name)
			continue
		}
		if s.Kind == "Group" {
			g = append(g, s.Name)
			continue
		}
	}
	role.SetKubeGroups(types.Allow, g)
	role.SetKubeUsers(types.Allow, u)

	if _, err := teleport.UpsertRole(
		context.Background(),
		&role,
	); err != nil {
		return trace.Wrap(err)
	}
	fmt.Println("Upserted Teleport role:", role.GetName())
	return nil
}

func createTeleportRolesForKubeCluster(teleport *client.Client, k types.KubeCluster) error {
	rbac, err := getRBACClient()
	if err != nil {
		return trace.Wrap(err)
	}

	crb, err := rbac.ClusterRoleBindings().List(
		context.Background(),
		metav1.ListOptions{},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, i := range crb.Items {
		if err := createTeleportRoleFromClusterRoleBinding(teleport, k, i); err != nil {
			return trace.Wrap(err)
		}
	}

	rb, err := rbac.RoleBindings("").List(
		context.Background(),
		metav1.ListOptions{},
	)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, i := range rb.Items {
		if err := createTeleportRoleFromRoleBinding(teleport, k, i); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(context.Background(), initTimeout)
	defer cancel()
	creds := client.LoadIdentityFile(identityFilePath)

	teleport, err := client.New(ctx, client.Config{
		Addrs:       []string{proxyAddr},
		Credentials: []client.Credentials{creds},
		DialOpts: []grpc.DialOption{
			grpc.WithReturnConnectionError(),
		},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("Connected to Teleport")

	ks, err := teleport.GetKubernetesServers(context.Background())
	if err != nil {
		panic(err)
	}
	for _, k := range ks {
		if k.GetCluster().GetName() != clusterName {
			continue
		}
		fmt.Println("Retrieved Kubernetes cluster", clusterName)

		if err := createTeleportRolesForKubeCluster(teleport, k.GetCluster()); err != nil {
			panic(err)
		}
		fmt.Println("Created roles for Kubernetes cluster", clusterName)
		return
	}
	panic("Unable to locate a Kubernetes Service instance for " + clusterName)
}
