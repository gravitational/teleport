// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package discovery

import (
	"slices"

	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
)

// resourceDescriptor is the single source of truth for how a discovery matcher
// type maps onto the summary output: its human-facing label, whether the
// backend reports detailed counts for it, the bucket those counts live in
// (also used to de-duplicate aggregation), and how to read them.
//
// Both the text block builder and the JSON/YAML digest builder describe matcher
// types through this type so the two outputs stay consistent.
type resourceDescriptor struct {
	displayName    string
	bucket         string
	supportsCounts bool
	counts         func(*discoveryconfig.IntegrationDiscoveredSummary) *discoveryconfigv1.ResourcesDiscoveredSummary
}

// lookupCounts returns the resource counts for this descriptor, guarding the
// nil summary case (the integration has no reported status).
func (d resourceDescriptor) lookupCounts(summary *discoveryconfig.IntegrationDiscoveredSummary) *discoveryconfigv1.ResourcesDiscoveredSummary {
	if summary == nil || d.counts == nil {
		return nil
	}
	return d.counts(summary)
}

func describeAWSResource(matcherType string) resourceDescriptor {
	switch {
	case matcherType == types.AWSMatcherEC2:
		return resourceDescriptor{
			displayName:    "EC2",
			bucket:         "aws_ec2",
			supportsCounts: true,
			counts: func(s *discoveryconfig.IntegrationDiscoveredSummary) *discoveryconfigv1.ResourcesDiscoveredSummary {
				return s.GetAwsEc2()
			},
		}
	case matcherType == types.AWSMatcherEKS:
		return resourceDescriptor{
			displayName:    "EKS",
			bucket:         "aws_eks",
			supportsCounts: true,
			counts: func(s *discoveryconfig.IntegrationDiscoveredSummary) *discoveryconfigv1.ResourcesDiscoveredSummary {
				return s.GetAwsEks()
			},
		}
	case slices.Contains(types.SupportedAWSDatabaseMatchers, matcherType):
		return resourceDescriptor{
			displayName:    "database",
			bucket:         "aws_rds",
			supportsCounts: true,
			counts: func(s *discoveryconfig.IntegrationDiscoveredSummary) *discoveryconfigv1.ResourcesDiscoveredSummary {
				return s.GetAwsRds()
			},
		}
	default:
		return resourceDescriptor{displayName: matcherType}
	}
}

func describeAzureResource(matcherType string) resourceDescriptor {
	switch matcherType {
	case types.AzureMatcherVM:
		return resourceDescriptor{
			displayName:    "VM",
			bucket:         "azure_vms",
			supportsCounts: true,
			counts: func(s *discoveryconfig.IntegrationDiscoveredSummary) *discoveryconfigv1.ResourcesDiscoveredSummary {
				return s.GetAzureVms()
			},
		}
	case types.AzureMatcherKubernetes:
		return resourceDescriptor{displayName: "AKS"}
	case types.AzureMatcherMySQL:
		return resourceDescriptor{displayName: "MySQL database"}
	case types.AzureMatcherPostgres:
		return resourceDescriptor{displayName: "PostgreSQL database"}
	case types.AzureMatcherRedis:
		return resourceDescriptor{displayName: "Redis database"}
	case types.AzureMatcherSQLServer:
		return resourceDescriptor{displayName: "SQL Server database"}
	default:
		return resourceDescriptor{displayName: matcherType}
	}
}
