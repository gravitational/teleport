/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/tbot/bot"
)

func TestArgoCDOutput_YAML(t *testing.T) {
	tests := []testYAMLCase[ArgoCDOutputConfig]{
		{
			name: "full",
			in: ArgoCDOutputConfig{
				Name: "my-argo-service",
				Selectors: []*KubernetesSelector{
					{
						Name:   "foo",
						Labels: map[string]string{},
					},
					{
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				},
				CredentialLifetime: bot.CredentialLifetime{
					TTL:             1 * time.Minute,
					RenewalInterval: 30 * time.Second,
				},
				SecretNamespace:  "argocd",
				SecretNamePrefix: "my-argo-cluster-",
				SecretLabels: map[string]string{
					"my-label": "value",
				},
				SecretAnnotations: map[string]string{
					"my-annotation": "value",
				},
				Project:             "super-secret-project",
				Namespaces:          []string{"prod", "dev"},
				ClusterResources:    true,
				ClusterNameTemplate: "{{.KubeName}}",
			},
		},
		{
			name: "minimal",
			in: ArgoCDOutputConfig{
				Selectors: []*KubernetesSelector{
					{
						Name:   "foo",
						Labels: map[string]string{},
					},
				},
			},
		},
	}
	testYAML(t, tests)
}

func TestArgoCDConfig_CheckAndSetDefaults(t *testing.T) {
	t.Setenv("POD_NAMESPACE", "my-pod-namespace")

	tests := []testCheckAndSetDefaultsCase[*ArgoCDOutputConfig]{
		{
			name: "valid_with_name_selector",
			in: func() *ArgoCDOutputConfig {
				return &ArgoCDOutputConfig{
					Selectors: []*KubernetesSelector{
						{Name: "foo", Labels: make(map[string]string)},
					},
					SecretNamespace:     "argocd",
					SecretNamePrefix:    "argo-cluster",
					ClusterNameTemplate: "{{.KubeName}}",
				}
			},
		},
		{
			name: "valid_with_labels",
			in: func() *ArgoCDOutputConfig {
				return &ArgoCDOutputConfig{
					Selectors: []*KubernetesSelector{
						{Labels: map[string]string{"foo": "bar"}},
					},
					SecretNamespace:     "argocd",
					SecretNamePrefix:    "argo-cluster",
					ClusterNameTemplate: "{{.KubeName}}",
				}
			},
		},
		{
			name: "no_selectors",
			in: func() *ArgoCDOutputConfig {
				return &ArgoCDOutputConfig{
					Selectors:           []*KubernetesSelector{},
					SecretNamespace:     "argocd",
					SecretNamePrefix:    "argo-cluster",
					ClusterNameTemplate: "{{.KubeName}}",
				}
			},
			wantErr: "at least one selector is required",
		},
		{
			name: "invalid_selectors",
			in: func() *ArgoCDOutputConfig {
				return &ArgoCDOutputConfig{
					Selectors: []*KubernetesSelector{
						{},
					},
					SecretNamespace:     "argocd",
					SecretNamePrefix:    "argo-cluster",
					ClusterNameTemplate: "{{.KubeName}}",
				}
			},
			wantErr: "one of 'name' and 'labels' must be specified",
		},
		{
			name: "invalid_secret_name_prefix",
			in: func() *ArgoCDOutputConfig {
				return &ArgoCDOutputConfig{
					Selectors: []*KubernetesSelector{
						{Labels: map[string]string{"foo": "bar"}},
					},
					SecretNamespace:     "argocd",
					SecretNamePrefix:    "NOT VALID",
					ClusterNameTemplate: "{{.KubeName}}",
				}
			},
			wantErr: "secret_name_prefix may only include lowercase letters, numbers, '-' and '.' characters",
		},
		{
			name: "empty_namespace",
			in: func() *ArgoCDOutputConfig {
				return &ArgoCDOutputConfig{
					Selectors: []*KubernetesSelector{
						{Labels: map[string]string{"foo": "bar"}},
					},
					SecretNamespace:     "argocd",
					SecretNamePrefix:    "argo-cluster",
					Namespaces:          []string{""},
					ClusterNameTemplate: "{{.KubeName}}",
				}
			},
			wantErr: "namespaces[0] cannot be blank",
		},
		{
			name: "invalid_namespaces",
			in: func() *ArgoCDOutputConfig {
				return &ArgoCDOutputConfig{
					Selectors: []*KubernetesSelector{
						{Labels: map[string]string{"foo": "bar"}},
					},
					SecretNamespace:     "argocd",
					SecretNamePrefix:    "argo-cluster",
					Namespaces:          []string{"foo,"},
					ClusterNameTemplate: "{{.KubeName}}",
				}
			},
			wantErr: "namespaces[0] is not a valid namespace name",
		},
		{
			name: "cluster_resources_but_no_namespaces",
			in: func() *ArgoCDOutputConfig {
				return &ArgoCDOutputConfig{
					Selectors: []*KubernetesSelector{
						{Labels: map[string]string{"foo": "bar"}},
					},
					SecretNamespace:     "argocd",
					SecretNamePrefix:    "argo-cluster",
					Namespaces:          []string{},
					ClusterResources:    true,
					ClusterNameTemplate: "{{.KubeName}}",
				}
			},
			wantErr: "cluster_resources is only applicable if namespaces is also set",
		},
		{
			name: "invalid cluster_name_template",
			in: func() *ArgoCDOutputConfig {
				return &ArgoCDOutputConfig{
					Selectors: []*KubernetesSelector{
						{Labels: map[string]string{"foo": "bar"}},
					},
					SecretNamespace:     "argocd",
					SecretNamePrefix:    "argo-cluster",
					ClusterNameTemplate: "{{.InvalidVariable}}",
				}
			},
			wantErr: "can't evaluate field InvalidVariable",
		},
		{
			name: "defaults",
			in: func() *ArgoCDOutputConfig {
				return &ArgoCDOutputConfig{
					Selectors: []*KubernetesSelector{
						{Labels: map[string]string{"foo": "bar"}},
					},
				}
			},
			want: &ArgoCDOutputConfig{
				Selectors: []*KubernetesSelector{
					{Labels: map[string]string{"foo": "bar"}},
				},
				SecretNamespace:     "my-pod-namespace",
				SecretNamePrefix:    "teleport.argocd-cluster",
				ClusterNameTemplate: "{{.ClusterName}}-{{.KubeName}}",
			},
		},
	}
	testCheckAndSetDefaults(t, tests)
}
