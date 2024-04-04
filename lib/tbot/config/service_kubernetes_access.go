package config

import (
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/bot"
)

const ServiceKubernetesType = "unstable-kubernetes"

// UnstableKubernetesService
//
// Limitations:
//
//   - Currently does not work with `tsh proxy kube` and hence will not work with
//     teleport clusters fronted with a L7 LB.
//   - Currently does not support the exec plugin for kubeconfig, which is
//     necessary to handle reloading the credentials mid-lifetime. We could also
//     magically resolve this with the tunnel functionality down the line rather
//     than relying on the exec plugin.
//
// Benefits:
//
//   - Supports multiple clusters.
type UnstableKubernetesService struct {
	Destination bot.Destination `yaml:"destination"`

	// Roles is the list of roles to request for the generated credentials.
	// If empty, it defaults to all the bot's roles.
	Roles []string `yaml:"roles,omitempty"`

	// KubernetesCluster is the name of the Kubernetes cluster in Teleport.
	// This is named a little more verbosely to avoid conflicting with the
	// name of the Teleport cluster to use.
	// Mutually exclusive with kubernetes_cluster_labels.
	KubernetesCluster       string            `yaml:"kubernetes_cluster"`
	KubernetesClusterLabels map[string]string `yaml:"kubernetes_cluster_labels"`
}

func (s *UnstableKubernetesService) Type() string {
	return ServiceKubernetesType
}

func (s *UnstableKubernetesService) MarshalYAML() (interface{}, error) {
	type raw UnstableKubernetesService
	return withTypeHeader((*raw)(s), ServiceKubernetesType)
}

func (s *UnstableKubernetesService) UnmarshalYAML(node *yaml.Node) error {
	dest, err := extractOutputDestination(node)
	if err != nil {
		return trace.Wrap(err)
	}
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw UnstableKubernetesService
	if err := node.Decode((*raw)(s)); err != nil {
		return trace.Wrap(err)
	}
	s.Destination = dest
	return nil
}

func (s *UnstableKubernetesService) CheckAndSetDefaults() error {
	if err := validateOutputDestination(s.Destination); err != nil {
		return trace.Wrap(err)
	}

	if s.KubernetesCluster == "" && len(s.KubernetesClusterLabels) == 0 {
		return trace.BadParameter("kubernetes_cluster or kubernetes_cluster_labels must be set")
	} else if s.KubernetesCluster != "" && len(s.KubernetesClusterLabels) > 0 {
		return trace.BadParameter("kubernetes_cluster and kubernetes_cluster_labels are mutually exclusive")
	}

	return nil
}
