package accessgraphui

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/arn"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
)

// ListIntegrationResponse is the response for listing integrations in the access graph.
type ListIntegrationResponse struct {
	Integrations []IsIntegration `json:"integrations"`
}

// IsIntegration is an interface for all types of integrations.
type IsIntegration interface {
	isIntegration()
}

// AWS is the AWS integration.
type AWS struct {
	// Kind is the kind of the integration.
	Kind string `json:"kind"`
	// SubKind is the sub kind of the integration.
	SubKind string `json:"subKind"`
	// Version is the version of the integration.
	Version string `json:"version"`
	// Metadata is the metadata of the integration.
	Metadata Metadata `json:"metadata"`
	// Spec is the specification of the integration.
	Spec Spec `json:"spec"`
	// Status is the status of the integration.
	Status IntegrationStatus `json:"status"`
}

func (*AWS) isIntegration() {}

// IntegrationStatus is the status of the integration.
type IntegrationStatus struct {
	// State is the current state of the discovery config.
	State string `json:"state"`
	// LastErrorMessage holds the error message when state is DISCOVERY_CONFIG_STATE_ERROR.
	LastErrorMessage *string `json:"last_error_message,omitempty"`
	// LastRawError variable stores the most recent raw error message received from an API or service.
	// It is intended to capture the original error message without any modifications or formatting.
	// This can be useful for debugging purposes, providing detailed information about what went wrong
	// in the interaction with the external service.
	LastRawError *string `json:"last_raw_error,omitempty"`
	// DiscoveredResources holds the count of the discovered resources in the previous iteration.
	DiscoveredResources uint64 `json:"discovered_resources"`
	// LastSyncTime is the timestamp when the Discovery Config was last sync.
	LastSyncTime time.Time `json:"last_sync_time"`
}

// Spec is the specification of the AWS integration.
type Spec struct {
	// AWSSpec is the list of AWS specifications.
	AWSSpec []AWSSpec `json:"aws"`
	// DiscoveryGroup is the discovery group of the integration.
	DiscoveryGroup string `json:"discoveryGroup"`
}

// AWSSpec is the AWS specification.
type AWSSpec struct {
	// AccountID is the account ID of the AWS integration.
	AccountID string `json:"accountID"`
	// Regions is the list of regions of the AWS integration.
	Regions []string `json:"regions"`
}

// Plugin is the plugin integration.
type Plugin struct {
	// Kind is the kind of the integration.
	Kind string `json:"kind"`
	// SubKind is the sub kind of the integration.
	SubKind string `json:"subKind"`
	// Version is the version of the integration.
	Version string `json:"version"`
	// Metadata is the metadata of the integration.
	Metadata Metadata `json:"metadata"`
	// Spec is the specification of the integration.
	Spec PluginSpec `json:"spec"`
	// Status is the status of the integration.
	Status PluginStatus `json:"status"`
}

func (*Plugin) isIntegration() {}

// PluginSpec is the specification of the plugin integration.
type PluginSpec struct {
	// Type is the type of the plugin.
	Type string `json:"type"`
	// Endpoint is the endpoint of the plugin.
	Endpoint string `json:"endpoint"`
}

// PluginStatus is the status of the plugin integration.
type PluginStatus struct {
	// State is the current state of the plugin.
	State string `json:"state"`
	// LastErrorMessage holds the error message.
	LastErrorMessage string `json:"last_error_message,omitempty"`
	// LastRawError variable stores the most recent raw error message received from an API or service.
	// It is intended to capture the original error message without any modifications or formatting.
	// This can be useful for debugging purposes, providing detailed information about what went wrong
	// in the interaction with the external service.
	LastRawError string `json:"last_raw_error,omitempty"`
	// LastSyncTime is the timestamp when the plugin was last sync.
	LastSyncTime time.Time `json:"last_sync_time"`
	// Details is the details of the plugin.
	Details *PluginDetails `json:"details,omitempty"`
}

// PluginDetails is the details of the plugin.
type PluginDetails struct {
	Gitlab *GitlabDetails `json:"gitlab,omitempty"`
	Entra  *EntraDetails  `json:"entra,omitempty"`
	NetIQ  *NetIQDetails  `json:"netiq,omitempty"`
}

// GitlabDetails is the details of the Gitlab plugin.
type GitlabDetails struct {
	// ImportedUsers is the count of imported users.
	ImportedUsers uint32 `json:"imported_users"`
	// ImportedGroups is the count of imported groups.
	ImportedGroups uint32 `json:"imported_groups"`
	// ImportedProjects is the count of imported projects.
	ImportedProjects uint32 `json:"imported_projects"`
}

// EntraDetails is the details of the Entra plugin.
type EntraDetails struct {
	// ImportedUsers is the count of imported users.
	ImportedUsers uint32 `json:"imported_users"`
	// ImportedGroups is the count of imported groups.
	ImportedGroups uint32 `json:"imported_groups"`
}

// NetIQDetails is the details of the NetIQ plugin.
type NetIQDetails struct {
	// ImportedUsers is the count of imported users.
	ImportedUsers uint32 `json:"imported_users"`
	// ImportedGroups is the count of imported groups.
	ImportedGroups uint32 `json:"imported_groups"`
	// ImportedRoles is the count of imported roles.
	ImportedRoles uint32 `json:"imported_roles"`
	// ImportedResources is the count of imported resources.
	ImportedResources uint32 `json:"imported_resources"`
}

// Metadata is the metadata of the integration.
type Metadata struct {
	// Name is the name of the integration.
	Name string `json:"name"`
	// Labels is the labels of the integration.
	Labels map[string]string `json:"labels"`
}

// MakeListIntegrationResponse creates a new ListIntegrationResponse.
func MakeListIntegrationResponse(dcs []*discoveryconfig.DiscoveryConfig, integrations []types.Integration, plugins []*types.PluginV1) *ListIntegrationResponse {
	integrationsMap := make(map[string]types.Integration)
	for _, i := range integrations {
		integrationsMap[i.GetName()] = i
	}
	var res ListIntegrationResponse
	for _, dc := range dcs {
		res.Integrations = append(res.Integrations, newIntegration(dc, integrationsMap))
	}
	for _, pl := range plugins {
		res.Integrations = append(res.Integrations, newPlugin(pl))
	}
	return &res
}

func newPlugin(pl *types.PluginV1) *Plugin {
	codeToStr := func(code types.PluginStatusCode) string {
		switch code {
		case types.PluginStatusCode_UNKNOWN:
			return "unknown"
		case types.PluginStatusCode_RUNNING:
			return "running"
		case types.PluginStatusCode_OTHER_ERROR:
			return "failing"
		case types.PluginStatusCode_UNAUTHORIZED:
			return "unauthorized"
		default:
			return fmt.Sprintf("unknown(%d)", code)
		}
	}
	var endpoint string
	if pl.Spec.GetOkta() != nil {
		endpoint = pl.Spec.GetOkta().OrgUrl
	} else if pl.Spec.GetGitlab() != nil {
		endpoint = pl.Spec.GetGitlab().ApiEndpoint
	} else if pl.Spec.GetNetIq() != nil {
		endpoint = pl.Spec.GetNetIq().ApiEndpoint
	}
	p := &Plugin{
		Kind:    pl.Kind,
		SubKind: pl.SubKind,
		Version: pl.Version,
		Metadata: Metadata{
			Name:   pl.Metadata.Name,
			Labels: pl.Metadata.Labels,
		},
		Spec: PluginSpec{
			Type:     string(pl.GetType()),
			Endpoint: endpoint,
		},
		Status: PluginStatus{
			State:            codeToStr(pl.Status.Code),
			LastErrorMessage: pl.Status.ErrorMessage,
			LastRawError:     pl.Status.LastRawError,
			LastSyncTime:     pl.Status.LastSyncTime,
		},
	}

	if gitlab := pl.Status.GetGitlab(); gitlab != nil {
		p.Status.Details = &PluginDetails{
			Gitlab: &GitlabDetails{
				ImportedUsers:    gitlab.ImportedUsers,
				ImportedGroups:   gitlab.ImportedGroups,
				ImportedProjects: gitlab.ImportedProjects,
			},
		}
	}
	if entra := pl.Status.GetEntraId(); entra != nil {
		p.Status.Details = &PluginDetails{
			Entra: &EntraDetails{
				ImportedUsers:  entra.ImportedUsers,
				ImportedGroups: entra.ImportedGroups,
			},
		}
	}
	if netIQ := pl.Status.GetNetIq(); netIQ != nil {
		p.Status.Details = &PluginDetails{
			NetIQ: &NetIQDetails{
				ImportedUsers:     netIQ.ImportedUsers,
				ImportedGroups:    netIQ.ImportedGroups,
				ImportedRoles:     netIQ.ImportedRoles,
				ImportedResources: netIQ.ImportedResources,
			},
		}
	}
	return p
}

func newIntegration(dc *discoveryconfig.DiscoveryConfig, integrations map[string]types.Integration) *AWS {
	var awsSpec []AWSSpec
	for _, aws := range dc.Spec.AccessGraph.AWS {
		awsSpec = append(awsSpec, AWSSpec{
			AccountID: getAWSAccountID(aws.AssumeRole, integrations[aws.Integration]),
			Regions:   aws.Regions,
		})
	}
	return &AWS{
		Kind:    dc.Kind,
		SubKind: dc.SubKind,
		Version: dc.Version,
		Metadata: Metadata{
			Name:   dc.Metadata.Name,
			Labels: dc.Metadata.Labels,
		},
		Spec: Spec{
			DiscoveryGroup: dc.Spec.DiscoveryGroup,
			AWSSpec:        awsSpec,
		},
		Status: IntegrationStatus{
			State:               dc.Status.State,
			LastErrorMessage:    dc.Status.ErrorMessage,
			DiscoveredResources: dc.Status.DiscoveredResources,
			LastSyncTime:        dc.Status.LastSyncTime,
		},
	}
}

func getAWSAccountID(assumeRole *types.AssumeRole, integration types.Integration) string {
	if assumeRole != nil {
		if arn, err := arn.Parse(assumeRole.RoleARN); err == nil {
			return arn.AccountID
		}
	}

	if integration != nil && integration.GetAWSOIDCIntegrationSpec() != nil {
		if arn, err := arn.Parse(integration.GetAWSOIDCIntegrationSpec().RoleARN); err == nil {
			return arn.AccountID
		}
	}

	return ""
}
