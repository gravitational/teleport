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
	discoveryconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/discoveryconfig"
)

// resourceDescriptor is the single source of truth for how a discovery matcher
// type maps onto the summary output: its human-facing label, whether the
// backend reports detailed counts for it, and how to read them.
type resourceDescriptor struct {
	displayName    string
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

var awsDatabaseDescriptor = resourceDescriptor{
	displayName:    "database",
	supportsCounts: true,
	counts: func(s *discoveryconfig.IntegrationDiscoveredSummary) *discoveryconfigv1.ResourcesDiscoveredSummary {
		return s.GetAwsRds()
	},
}

// displayName values are rendered CLI output strings; keep them byte-compatible.
var awsResourceDescriptors = map[string]resourceDescriptor{
	types.AWSMatcherEC2: {
		displayName:    "EC2",
		supportsCounts: true,
		counts: func(s *discoveryconfig.IntegrationDiscoveredSummary) *discoveryconfigv1.ResourcesDiscoveredSummary {
			return s.GetAwsEc2()
		},
	},
	types.AWSMatcherEKS: {
		displayName:    "EKS",
		supportsCounts: true,
		counts: func(s *discoveryconfig.IntegrationDiscoveredSummary) *discoveryconfigv1.ResourcesDiscoveredSummary {
			return s.GetAwsEks()
		},
	},
	types.AWSMatcherRDS:                   awsDatabaseDescriptor,
	types.AWSMatcherRDSProxy:              awsDatabaseDescriptor,
	types.AWSMatcherRedshift:              awsDatabaseDescriptor,
	types.AWSMatcherRedshiftServerless:    awsDatabaseDescriptor,
	types.AWSMatcherElastiCache:           awsDatabaseDescriptor,
	types.AWSMatcherElastiCacheServerless: awsDatabaseDescriptor,
	types.AWSMatcherMemoryDB:              awsDatabaseDescriptor,
	types.AWSMatcherOpenSearch:            awsDatabaseDescriptor,
	types.AWSMatcherDocumentDB:            awsDatabaseDescriptor,
}

func describeAWSResource(matcherType string) resourceDescriptor {
	if desc, ok := awsResourceDescriptors[matcherType]; ok {
		return desc
	}
	return resourceDescriptor{displayName: matcherType}
}

// displayName values are rendered CLI output strings; keep them byte-compatible.
var azureResourceDescriptors = map[string]resourceDescriptor{
	types.AzureMatcherVM: {
		displayName:    "VM",
		supportsCounts: true,
		counts: func(s *discoveryconfig.IntegrationDiscoveredSummary) *discoveryconfigv1.ResourcesDiscoveredSummary {
			return s.GetAzureVms()
		},
	},
	types.AzureMatcherKubernetes: {displayName: "AKS"},
	types.AzureMatcherMySQL:      {displayName: "MySQL database"},
	types.AzureMatcherPostgres:   {displayName: "PostgreSQL database"},
	types.AzureMatcherRedis:      {displayName: "Redis database"},
	types.AzureMatcherSQLServer:  {displayName: "SQL Server database"},
}

func describeAzureResource(matcherType string) resourceDescriptor {
	if desc, ok := azureResourceDescriptors[matcherType]; ok {
		return desc
	}
	return resourceDescriptor{displayName: matcherType}
}
