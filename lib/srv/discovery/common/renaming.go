/*
Copyright 2023 Gravitational, Inc.

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

package common

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
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
	suffix := makeAWSDiscoverySuffix(databaseNameValidator,
		db.GetName(),
		matcherType,
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
	suffix := makeAzureDiscoverySuffix(databaseNameValidator,
		db.GetName(),
		matcherType,
		region,
		group,
		subID,
	)
	applyDiscoveryNameSuffix(db, suffix)
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
	suffix := makeAWSDiscoverySuffix(kubeClusterNameValidator,
		cluster.GetName(),
		services.AWSMatcherEKS,
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
	suffix := makeAzureDiscoverySuffix(kubeClusterNameValidator,
		cluster.GetName(),
		services.AzureMatcherKubernetes,
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
	suffix := makeGCPDiscoverySuffix(kubeClusterNameValidator,
		cluster.GetName(),
		services.GCPMatcherKubernetes,
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
// form <matcher type>-<region>-<account ID>.
func makeAWSDiscoverySuffix(fn suffixValidatorFn, name, matcherType, region, accountID string) string {
	return makeDiscoverySuffix(fn, name, matcherType, region, accountID)
}

// makeAzureDiscoverySuffix makes a discovery suffix for Azure resources, of the
// form <matcher type>-<region>-<resource group>-<subscription ID>.
func makeAzureDiscoverySuffix(fn suffixValidatorFn, name, matcherType, region, resourceGroup, subscriptionID string) string {
	return makeDiscoverySuffix(fn, name, matcherType, region, resourceGroup, subscriptionID)
}

// makeGCPDiscoverySuffix makes a discovery suffix for GCP resources, of the
// form <matcher type>-<location>-<project ID>.
func makeGCPDiscoverySuffix(fn suffixValidatorFn, name, matcherType, location, projectID string) string {
	return makeDiscoverySuffix(fn, name, matcherType, location, projectID)
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

// databaseNameValidator is a suffixValidatorFn for databases.
func databaseNameValidator(part string) error {
	// validate the suffix part adding a simple stub prefix "a" and
	// validating it as a full database name.
	return types.ValidateDatabaseName("a" + part)
}

// kubeClusterNameValidator is a suffixValidatorFn for kube clusters.
func kubeClusterNameValidator(part string) error {
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
