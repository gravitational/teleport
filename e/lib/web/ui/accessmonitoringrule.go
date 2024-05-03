package ui

import (
	yaml "github.com/ghodss/yaml"
	"github.com/gravitational/trace"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
)

type AccessMonitoringRulePage struct {
	Rules    []AccessMonitoringRuleWithYaml `json:"rules"`
	StartKey string                         `json:"startKey"`
}

type AccessMonitoringRuleWithYaml struct {
	Object *accessmonitoringrulesv1.AccessMonitoringRule `json:"object"`
	// YAML is resource yaml content.
	YAML string `json:"yaml"`
}

func MakeAccessMonitoringRuleWithYamlContent(resource *accessmonitoringrulesv1.AccessMonitoringRule) (AccessMonitoringRuleWithYaml, error) {
	data, err := yaml.Marshal(resource)
	if err != nil {
		return AccessMonitoringRuleWithYaml{}, trace.Wrap(err)
	}

	return AccessMonitoringRuleWithYaml{
		Object: resource,
		YAML:   string(data),
	}, nil
}
