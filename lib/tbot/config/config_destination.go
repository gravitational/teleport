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

package config

import (
	"reflect"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
)

// DatabaseConfig is the config for a database access request.
type DatabaseConfig struct {
	// Service is the service name of the Teleport database. Generally this is
	// the name of the Teleport resource.
	Service string `yaml:"service,omitempty"`

	// Database is the name of the database to request access to.
	Database string `yaml:"database,omitempty"`

	// Username is the database username to request access as.
	Username string `yaml:"username,omitempty"`
}

func (dc *DatabaseConfig) CheckAndSetDefaults() error {
	if dc.Service == "" {
		return trace.BadParameter("database `service` field must specify a database service name")
	}

	// Note: tsh has special checks for MongoDB and Redis. We don't know the
	// protocol at this point so we'll need to defer those checks.

	return nil
}

// KubernetesCluster is a Kubernetes cluster certificate request.
type KubernetesCluster struct {
	// ClusterName is the name of the Kubernetes cluster in Teleport.
	ClusterName string
}

func (kc *KubernetesCluster) UnmarshalYAML(node *yaml.Node) error {
	// We don't care for multiple YAML shapes here, we just want our Kubernetes
	// config field to be compatible with CheckAndSetDefaults().

	var clusterName string
	if err := node.Decode(&clusterName); err != nil {
		return trace.Wrap(err)
	}

	kc.ClusterName = clusterName
	return nil
}

func (kc *KubernetesCluster) MarshalYAML() (interface{}, error) {
	return kc.ClusterName, nil
}

func (kc *KubernetesCluster) CheckAndSetDefaults() error {
	if kc.ClusterName == "" {
		return trace.BadParameter("Kubernetes cluster name must not be empty")
	}

	return nil
}

// DestinationConfig configures a user certificate destination.
type DestinationConfig struct {
	DestinationMixin `yaml:",inline"`

	Roles   []string         `yaml:"roles,omitempty"`
	Configs []TemplateConfig `yaml:"configs,omitempty"`

	// Kinds is a deprecated and unused field that remains for compatibility
	// reasons.
	// DELETE IN 11.0.0: Kinds should be removed after a grace period.
	Kinds []string `yaml:"kinds,omitempty"`

	// Database is a database to request access to. Mutually exclusive with
	// `kubernetes_cluster` and other special cert requests.
	Database *DatabaseConfig `yaml:"database,omitempty"`

	// KubernetesCluster is a cluster to request access to. Mutually exclusive
	// with `database` and other special cert requests.
	KubernetesCluster *KubernetesCluster `yaml:"kubernetes_cluster,omitempty"`
}

// destinationDefaults applies defaults for an output sink's destination. Since
// these have no sane defaults, in practice it just returns an error if no
// config is provided.
func destinationDefaults(dm *DestinationMixin) error {
	return trace.BadParameter("destinations require some valid output sink")
}

// addRequiredConfigs adds all configs with default parameters that were not
// explicitly requested by users. Several configs, including `identity`, `tls`,
// and `ssh_client`, are always generated (with defaults set, if any) but will
// not be overridden if already included by the user.
func (dc *DestinationConfig) addRequiredConfigs() {
	if dc.GetConfigByName(TemplateSSHClientName) == nil {
		dc.Configs = append(dc.Configs, TemplateConfig{
			SSHClient: &TemplateSSHClient{},
		})
	}

	if dc.GetConfigByName(TemplateIdentityName) == nil {
		dc.Configs = append(dc.Configs, TemplateConfig{
			Identity: &TemplateIdentity{},
		})
	}

	if dc.GetConfigByName(TemplateTLSCAsName) == nil {
		dc.Configs = append(dc.Configs, TemplateConfig{
			TLSCAs: &TemplateTLSCAs{},
		})
	}

	// If a k8s request exists, enable the kubernetes template.
	if dc.KubernetesCluster != nil && dc.GetConfigByName(TemplateKubernetesName) == nil {
		dc.Configs = append(dc.Configs, TemplateConfig{
			Kubernetes: &TemplateKubernetes{},
		})
	}
}

func (dc *DestinationConfig) CheckAndSetDefaults() error {
	if err := dc.DestinationMixin.CheckAndSetDefaults(destinationDefaults); err != nil {
		return trace.Wrap(err)
	}

	certRequests := []interface{ CheckAndSetDefaults() error }{
		dc.Database,
		dc.KubernetesCluster,
	}
	notNilCount := 0
	for _, request := range certRequests {
		// Note: this check is fragile and will fail if the templates aren't
		// all simple pointer types. They are, though, and the "correct"
		// solution is insane, so we'll stick with this.
		if reflect.ValueOf(request).IsNil() {
			continue
		}

		if request != nil {
			if err := request.CheckAndSetDefaults(); err != nil {
				return trace.Wrap(err)
			}

			notNilCount++
		}
	}

	if notNilCount > 1 {
		return trace.BadParameter("a destination can make at most one " +
			"special certificate request (database, kubernetes_cluster, etc)")
	}

	// Note: empty roles is allowed; interpreted to mean "all" at generation
	// time

	dc.addRequiredConfigs()

	for _, cfg := range dc.Configs {
		if err := cfg.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
	}

	if len(dc.Kinds) > 0 {
		log.Warnf("The `kinds` configuration field has been deprecated and " +
			"will be removed in a future release. It is now a no-op and can " +
			"safely be removed from the configuration file.")
	}

	return nil
}

// ListSubdirectories lists all subdirectories that should be contained within
// this destination. Primarily used for on-the-fly directory creation.
func (dc *DestinationConfig) ListSubdirectories() ([]string, error) {
	// Note: currently no standard identity.Artifacts create subdirs. If that
	// ever changes, we'll need to adapt this to ensure we initialize them
	// properly on the fly.
	var subdirs []string

	dest, err := dc.GetDestination()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, config := range dc.Configs {
		template, err := config.GetConfigTemplate()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, file := range template.Describe(dest) {
			if file.IsDir {
				subdirs = append(subdirs, file.Name)
			}
		}
	}

	return subdirs, nil
}

// GetConfigByName returns the first valid template with the given name
// contained within this destination.
func (dc *DestinationConfig) GetConfigByName(name string) Template {
	for _, cfg := range dc.Configs {
		tpl, err := cfg.GetConfigTemplate()
		if err != nil {
			continue
		}

		if tpl.Name() == name {
			return tpl
		}
	}

	return nil
}

// GetRequiredConfig returns the static list of all default / required config
// templates.
func GetRequiredConfigs() []string {
	return []string{TemplateTLSCAsName, TemplateSSHClientName, TemplateIdentityName}
}
