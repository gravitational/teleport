/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package common

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gravitational/teleport/api/types"
)

// ApplyAWSDatabaseNameSuffix applies the AWS Database Discovery name suffix to
// the given database.
// Format: <name>-<matcher type>-<region>-<account ID>.
func ApplyAWSDatabaseNameSuffix(db types.Database, matcherType string) {
	if hasOverrideLabel(db, types.AWSDatabaseNameOverrideLabels...) {
		// don't rewrite manual name override.
		return
	}
	meta := db.GetAWS()
	suffix := makeAWSDiscoverySuffix(databaseNamePartValidator,
		db.GetName(),
		matcherType,
		getDBMatcherSubtype(matcherType, db),
		meta.Region,
		meta.AccountID,
	)
	applyDiscoveryNameSuffix(db, suffix)
}

// ApplyAzureDatabaseNameSuffix applies the Azure Database Discovery name suffix
// to the given database.
// Format: <name>-<matcher type>-<region>-<resource group>-<subscription ID>.
func ApplyAzureDatabaseNameSuffix(db types.Database, matcherType string) {
	if hasOverrideLabel(db, types.AzureDatabaseNameOverrideLabel) {
		// don't rewrite manual name override.
		return
	}
	region, _ := db.GetLabel(types.DiscoveryLabelRegion)
	group, _ := db.GetLabel(types.DiscoveryLabelAzureResourceGroup)
	subID, _ := db.GetLabel(types.DiscoveryLabelAzureSubscriptionID)
	suffix := makeAzureDiscoverySuffix(databaseNamePartValidator,
		db.GetName(),
		matcherType,
		getDBMatcherSubtype(matcherType, db),
		region,
		group,
		subID,
	)
	applyDiscoveryNameSuffix(db, suffix)
}

// getDBMatcherSubtype gets a "subtype" for a given DB matcher, based on the
// database metadata. This is needed for AWS RDS and Azure Redis databases
// to ensure unique naming.
// For example, an Aurora cluster can be named the same as an RDS instance in
// the same account, region, etc.
// Likewise, an Azure Redis database can be named the same as an Azure Redis
// Enterprise database.
// By subtyping the matcher type, we can ensure these names do not collide.
func getDBMatcherSubtype(matcherType string, db types.Database) string {
	switch matcherType {
	case types.AWSMatcherRDS:
		if db.GetAWS().RDS.InstanceID == "" {
			// distinguish RDS instances from clusters by subtyping the RDS
			// matcher as "rds-aurora".
			return "aurora"
		}
	case types.AzureMatcherRedis:
		if db.GetAzure().Redis.ClusteringPolicy != "" {
			// distinguish Redis databases from Redis Enterprise database by
			// subtyping the redis matcher as "redis-enterprise".
			return "enterprise"
		}
	}
	return ""
}

// ApplyEKSNameSuffix applies the AWS EKS Discovery name suffix to the given
// kube cluster.
// Format: <name>-eks-<region>-<account ID>.
func ApplyEKSNameSuffix(cluster types.KubeCluster) {
	if hasOverrideLabel(cluster, types.AWSKubeClusterNameOverrideLabels...) {
		// don't rewrite manual name override.
		return
	}
	meta := cluster.GetAWSConfig()
	suffix := makeAWSDiscoverySuffix(kubeClusterNamePartValidator,
		cluster.GetName(),
		types.AWSMatcherEKS,
		"", // no EKS subtype
		meta.Region,
		meta.AccountID,
	)
	applyDiscoveryNameSuffix(cluster, suffix)
}

// ApplyAKSNameSuffix applies the Azure AKS Discovery name suffix to the given
// kube cluster.
// Format: <name>-aks-<region>-<resource group>-<subscription ID>.
func ApplyAKSNameSuffix(cluster types.KubeCluster) {
	if hasOverrideLabel(cluster, types.AzureKubeClusterNameOverrideLabel) {
		// don't rewrite manual name override.
		return
	}
	meta := cluster.GetAzureConfig()
	region, _ := cluster.GetLabel(types.DiscoveryLabelRegion)
	suffix := makeAzureDiscoverySuffix(kubeClusterNamePartValidator,
		cluster.GetName(),
		types.AzureMatcherKubernetes,
		"", // no AKS subtype
		region,
		meta.ResourceGroup,
		meta.SubscriptionID,
	)
	applyDiscoveryNameSuffix(cluster, suffix)
}

// ApplyGKENameSuffix applies the GCP GKE Discovery name suffix to the given
// kube cluster.
// Format: <name>-gke-<location>-<project ID>.
func ApplyGKENameSuffix(cluster types.KubeCluster) {
	if hasOverrideLabel(cluster, types.GCPKubeClusterNameOverrideLabel) {
		// don't rewrite manual name override.
		return
	}
	meta := cluster.GetGCPConfig()
	suffix := makeGCPDiscoverySuffix(kubeClusterNamePartValidator,
		cluster.GetName(),
		types.GCPMatcherKubernetes,
		"", // no GKE subtype
		meta.Location,
		meta.ProjectID,
	)
	applyDiscoveryNameSuffix(cluster, suffix)
}

// hasOverrideLabel is a helper func to check for presence of a name override
// label.
func hasOverrideLabel(r types.ResourceWithLabels, overrideLabels ...string) bool {
	for _, label := range overrideLabels {
		if val, ok := r.GetLabel(label); ok && val != "" {
			return true
		}
	}
	return false
}

// makeAWSDiscoverySuffix makes a discovery suffix for AWS resources, of the
// form <matcher type>-<subtype>-<region>-<account ID>.
func makeAWSDiscoverySuffix(fn suffixValidatorFn, name, matcherType, subType, region, accountID string) string {
	return makeDiscoverySuffix(fn, name, matcherType, subType, region, accountID)
}

// makeAzureDiscoverySuffix makes a discovery suffix for Azure resources, of the
// form <matcher type>-<subtype>-<region>-<resource group>-<subscription ID>.
func makeAzureDiscoverySuffix(fn suffixValidatorFn, name, matcherType, subType, region, resourceGroup, subscriptionID string) string {
	return makeDiscoverySuffix(fn, name, matcherType, subType, region, resourceGroup, subscriptionID)
}

// makeGCPDiscoverySuffix makes a discovery suffix for GCP resources, of the
// form <matcher type>-<subtype>-<location>-<project ID>.
func makeGCPDiscoverySuffix(fn suffixValidatorFn, name, matcherType, subType, location, projectID string) string {
	return makeDiscoverySuffix(fn, name, matcherType, subType, location, projectID)
}

// applyDiscoveryNameSuffix takes a resource with labels and a suffix to add
// to the name, then modifies the resource to add a label containing the
// original name and sets a new name with the suffix appended.
// This function does nothing if the suffix is empty.
func applyDiscoveryNameSuffix(resource types.ResourceWithLabels, suffix string) {
	if suffix == "" {
		// nop if suffix parts aren't given.
		return
	}
	discoveredName := resource.GetName()
	labels := resource.GetStaticLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	// set the originally discovered name as a label.
	labels[types.DiscoveredNameLabel] = discoveredName
	resource.SetStaticLabels(labels)
	// update the resource name with a suffix.
	resource.SetName(fmt.Sprintf("%s-%s", discoveredName, suffix))
}

// suffixValidatorFn is a func that validates a suffix.
type suffixValidatorFn func(string) error

// databaseNamePartValidator is a suffixValidatorFn for database name suffix
// parts.
func databaseNamePartValidator(part string) error {
	// validate the suffix part adding a simple stub prefix "a" and
	// validating it as a full database name.
	return types.ValidateDatabaseName("a" + part)
}

// kubeClusterNamePartValidator is a suffixValidatorFn for kube cluster suffix
// parts.
func kubeClusterNamePartValidator(part string) error {
	// validate the suffix part adding a simple stub prefix "a" and
	// validating it as a full kube cluster name.
	return types.ValidateKubeClusterName("a" + part)
}

// makeDiscoverySuffix takes a list of suffix parts and a suffix validator func,
// sanitizes each part and checks it for validity, then joins the result with
// hyphens "-".
func makeDiscoverySuffix(validatorFn suffixValidatorFn, name string, parts ...string) string {
	// convert name to lower case for substring checking.
	name = strings.ToLower(name)
	var out []string
	for _, part := range parts {
		part = sanitizeSuffixPart(part)
		// skip blank parts.
		if part == "" {
			continue
		}
		// skip redundant parts.
		if strings.Contains(name, strings.ToLower(part)) {
			continue
		}
		// skip invalid parts.
		if err := validatorFn(part); err != nil {
			continue
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return ""
	}
	suffix := strings.Join(out, "-")
	if err := validatorFn(suffix); err != nil {
		// sanity check for the full suffix - if it's somehow invalid, then
		// discard it.
		return ""
	}
	return suffix
}

// sanitizeSuffixPart cleans a suffix part to remove all whitespace, repeating
// and leading/trailing hyphens, and converts the string to all lowercase.
func sanitizeSuffixPart(part string) string {
	// convert all whitespace to "-".
	part = strings.ReplaceAll(part, " ", "-")
	// compact repeating "-" into a single "-", e.g. "a--b----" => "a-b-".
	part = repeatingHyphensRegexp.ReplaceAllLiteralString(part, "-")
	// trim leading/trailing hyphens out.
	part = strings.Trim(part, "-")
	return part
}

// repeatingHyphensRegexp represents a repeating hyphen chars pattern.
var repeatingHyphensRegexp = regexp.MustCompile(`--+`)
