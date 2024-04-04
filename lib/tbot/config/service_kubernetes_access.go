package config

import (
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
)

const ServiceKubernetesType = "kubernetes"

type KubernetesServiceFormat string

const (
	KubernetesServiceFormatNone                 KubernetesServiceFormat = ""
	KubernetesServiceFormatKubeconfig           KubernetesServiceFormat = "kubeconfig"
	KubernetesServiceFormatKubeconfigExecPlugin KubernetesServiceFormat = "kubeconfig-exec-plugin"
	KubernetesServiceFormatArgoCD               KubernetesServiceFormat = "argocd"
)

// KubernetesService
type KubernetesService struct {
	// TODO:
	// Destination bot.Destination `yaml:"destination"`

	// Roles is the list of roles to request for the generated credentials.
	// If empty, it defaults to all the bot's roles.
	Roles []string `yaml:"roles,omitempty"`

	Format KubernetesServiceFormat `yaml:"format"`

	// KubernetesCluster is the name of the Kubernetes cluster in Teleport.
	// This is named a little more verbosely to avoid conflicting with the
	// name of the Teleport cluster to use.
	// Mutually exclusive with kubernetes_cluster_labels.
	KubernetesCluster       string            `yaml:"kubernetes_cluster"`
	KubernetesClusterLabels map[string]string `yaml:"kubernetes_cluster_labels"`
}

func (s *KubernetesService) Type() string {
	return ServiceKubernetesType
}

func (s *KubernetesService) MarshalYAML() (interface{}, error) {
	type raw KubernetesService
	return withTypeHeader((*raw)(s), ServiceKubernetesType)
}

func (s *KubernetesService) UnmarshalYAML(node *yaml.Node) error {
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw KubernetesService
	if err := node.Decode((*raw)(s)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *KubernetesService) CheckAndSetDefaults() error {
	return nil
}
