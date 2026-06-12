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
	"cmp"
	"os"

	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/api/validation"

	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/internal/encoding"
)

const ArgoCDOutputServiceType = "kubernetes/argo-cd"

var defaultContextNameTemplate = kubeconfig.ContextName("{{.ClusterName}}", "{{.KubeName}}")

// ArgoCDOutputConfig contains configuration for the service that registers
// Kubernetes cluster credentials in Argo CD.
//
// See: https://argo-cd.readthedocs.io/en/stable/operator-manual/declarative-setup/#clusters
type ArgoCDOutputConfig struct {
	// Name of the service for logs and the /readyz endpoint.
	Name string `yaml:"name,omitempty"`

	// CredentialLifetime contains configuration for how long credentials will
	// last and the frequency at which they'll be renewed.
	CredentialLifetime bot.CredentialLifetime `yaml:",inline"`

	// Selectors is a list of selectors used to determine which Kubernetes
	// clusters will be registered in Argo CD.
	Selectors []*KubernetesSelector `yaml:"selectors,omitempty"`

	// SecretNamespace is the namespace to which cluster secrets will be written.
	// By default, it will use the `POD_NAMESPACE` environment variable, or if
	// that is empty: "default".
	SecretNamespace string `yaml:"secret_namespace,omitempty"`

	// SecretNamePrefix is the prefix that will be applied to Kubernetes secret
	// names. The rest of the name will be derived from the name of the target
	// cluster. Defaults to: "teleport.argocd-cluster".
	SecretNamePrefix string `yaml:"secret_name_prefix,omitempty"`

	// SecretLabels is a set of labels that will be applied to the created
	// Kubernetes secrets (in addition to the labels added for Argo's benefit).
	SecretLabels map[string]string `yaml:"secret_labels,omitempty"`

	// SecretLabels is a set of annotations that will be applied to the created
	// Kubernetes secrets (in addition to tbot's own annotations).
	SecretAnnotations map[string]string `yaml:"secret_annotations,omitempty"`

	// Project is the Argo CD project with which the Kubernetes cluster
	// credentials will be associated.
	Project string `yaml:"project,omitempty"`

	// Namespaces are the Kubernetes namespaces the created Argo CD cluster
	// credentials will be allowed to operate on.
	Namespaces []string `yaml:"namespaces,omitempty"`

	// ClusterResources determines whether the created Argo CD cluster
	// credentials will be allowed to operate on cluster-scoped resources (only
	// when Namespaces is non-empty).
	ClusterResources bool `yaml:"cluster_resources,omitempty"`

	// ClusterNameTemplate determines the format of cluster names in Argo CD.
	// It is a "text/template" string that supports the following variables:
	//
	//   - {{.ClusterName}} - Name of the Teleport cluster
	//   - {{.KubeName}} - Name of the Kubernetes cluster resource
	//
	// By default, the following template will be used: "{{.ClusterName}}-{{.KubeName}}".
	ClusterNameTemplate string `yaml:"cluster_name_template,omitempty"`
}

// GetName returns the user-given name of the service, used for validation purposes.
func (o *ArgoCDOutputConfig) GetName() string {
	return o.Name
}

// SetName sets the service's name to an automatically generated one.
func (o *ArgoCDOutputConfig) SetName(name string) {
	o.Name = name
}

// CheckAndSetDefaults validates the service configuration and sets any default
// values.
func (o *ArgoCDOutputConfig) CheckAndSetDefaults() error {
	if len(o.Selectors) == 0 {
		return trace.BadParameter("at least one selector is required")
	}
	for idx, selector := range o.Selectors {
		if err := selector.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err, "validating selectors[%d]", idx)
		}
	}

	o.SecretNamespace = cmp.Or(
		o.SecretNamespace,
		os.Getenv(kubernetesNamespaceEnv),
		"default",
	)

	if o.SecretNamePrefix == "" {
		o.SecretNamePrefix = "teleport.argocd-cluster"
	} else {
		if len(validation.NameIsDNSSubdomain(o.SecretNamePrefix, true)) != 0 {
			return trace.BadParameter("secret_name_prefix may only include lowercase letters, numbers, '-' and '.' characters")
		}
	}

	for idx, ns := range o.Namespaces {
		if ns == "" {
			return trace.BadParameter("namespaces[%d] cannot be blank", idx)
		}
		if len(validation.ValidateNamespaceName(ns, false)) != 0 {
			return trace.BadParameter("namespaces[%d] is not a valid namespace name", idx)
		}
	}

	if o.ClusterResources && len(o.Namespaces) == 0 {
		return trace.BadParameter("cluster_resources is only applicable if namespaces is also set")
	}

	if o.ClusterNameTemplate == "" {
		o.ClusterNameTemplate = defaultContextNameTemplate
	} else {
		if _, err := kubeconfig.ContextNameFromTemplate(o.ClusterNameTemplate, "", ""); err != nil {
			return trace.BadParameter("cluster_name_template is invalid: %v", err)
		}
	}

	return nil
}

// Type returns the service type string.
func (s *ArgoCDOutputConfig) Type() string {
	return ArgoCDOutputServiceType
}

// MarshalYAML marshals the configuration to YAML.
func (s *ArgoCDOutputConfig) MarshalYAML() (any, error) {
	type raw ArgoCDOutputConfig
	return encoding.WithTypeHeader((*raw)(s), ArgoCDOutputServiceType)
}

// GetCredentialLifetime returns the service's credential lifetime.
func (o *ArgoCDOutputConfig) GetCredentialLifetime() bot.CredentialLifetime {
	return o.CredentialLifetime
}
