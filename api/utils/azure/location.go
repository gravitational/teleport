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

package azure

import "strings"

// NormalizeLocation converts a Azure location in various formats to the same
// simple format.
//
// This function assumes the input location is in one of the following formats:
// - Name (the "simple" format): "northcentralusstage"
// - Display name: "North Central US (Stage)"
// - Regional display name: "(US) North Central US (Stage)"
//
// Note that the location list can be generated from `az account list-locations
// -o table`. However, this CLI command only lists the locations for the
// current active subscription so it may not show locations in other
// parititions like Government or China.
func NormalizeLocation(input string) string {
	if input == "" {
		return input
	}

	// Check if the input is a recognized simple name.
	if _, found := locationsToDisplayNames[input]; found {
		return input
	}

	// If input starts with '(', it should be the Regional display name. Then
	// removes the first bracket and its content. The leftover will be a
	// display name.
	if input[0] == '(' {
		if index := strings.IndexRune(input, ')'); index >= 0 {
			input = input[index:]
		}
		input = strings.TrimSpace(input)
	}

	// Check if the input is a recognized display name. If so, return the
	// simple name from the mapping.
	if location, found := displayNamesToLocations[input]; found {
		return location
	}

	// Try our best to convert an unregconized input:
	// - Remove brackets and spaces.
	// - To lower case.
	replacer := strings.NewReplacer("(", "", ")", "", " ", "")
	return strings.ToLower(replacer.Replace(input))
}

// GetLocationDisplayName returns the display name of the location.
func GetLocationDisplayName(location string) string {
	if displayName, found := locationsToDisplayNames[location]; found {
		return displayName
	}
	// Return the original input if not found.
	return location
}

var (
	// displayNamesToLocations maps a location's "Display Name" to its simple
	// "Name".
	displayNamesToLocations = map[string]string{
		// Azure locations.
		"East US":                  "eastus",
		"East US 2":                "eastus2",
		"South Central US":         "southcentralus",
		"West US 2":                "westus2",
		"West US 3":                "westus3",
		"Australia East":           "australiaeast",
		"Southeast Asia":           "southeastasia",
		"North Europe":             "northeurope",
		"Sweden Central":           "swedencentral",
		"UK South":                 "uksouth",
		"West Europe":              "westeurope",
		"Central US":               "centralus",
		"South Africa North":       "southafricanorth",
		"Central India":            "centralindia",
		"East Asia":                "eastasia",
		"Japan East":               "japaneast",
		"Korea Central":            "koreacentral",
		"Canada Central":           "canadacentral",
		"France Central":           "francecentral",
		"Germany West Central":     "germanywestcentral",
		"Norway East":              "norwayeast",
		"Switzerland North":        "switzerlandnorth",
		"UAE North":                "uaenorth",
		"Brazil South":             "brazilsouth",
		"East US 2 EUAP":           "eastus2euap",
		"Qatar Central":            "qatarcentral",
		"Central US (Stage)":       "centralusstage",
		"East US (Stage)":          "eastusstage",
		"East US 2 (Stage)":        "eastus2stage",
		"North Central US (Stage)": "northcentralusstage",
		"South Central US (Stage)": "southcentralusstage",
		"West US (Stage)":          "westusstage",
		"West US 2 (Stage)":        "westus2stage",
		"Asia":                     "asia",
		"Asia Pacific":             "asiapacific",
		"Australia":                "australia",
		"Brazil":                   "brazil",
		"Canada":                   "canada",
		"Europe":                   "europe",
		"France":                   "france",
		"Germany":                  "germany",
		"Global":                   "global",
		"India":                    "india",
		"Japan":                    "japan",
		"Korea":                    "korea",
		"Norway":                   "norway",
		"Singapore":                "singapore",
		"South Africa":             "southafrica",
		"Switzerland":              "switzerland",
		"United Arab Emirates":     "uae",
		"United Kingdom":           "uk",
		"United States":            "unitedstates",
		"United States EUAP":       "unitedstateseuap",
		"East Asia (Stage)":        "eastasiastage",
		"Southeast Asia (Stage)":   "southeastasiastage",
		"East US STG":              "eastusstg",
		"South Central US STG":     "southcentralusstg",
		"North Central US":         "northcentralus",
		"West US":                  "westus",
		"Jio India West":           "jioindiawest",
		"Central US EUAP":          "centraluseuap",
		"West Central US":          "westcentralus",
		"South Africa West":        "southafricawest",
		"Australia Central":        "australiacentral",
		"Australia Central 2":      "australiacentral2",
		"Australia Southeast":      "australiasoutheast",
		"Japan West":               "japanwest",
		"Jio India Central":        "jioindiacentral",
		"Korea South":              "koreasouth",
		"South India":              "southindia",
		"West India":               "westindia",
		"Canada East":              "canadaeast",
		"France South":             "francesouth",
		"Germany North":            "germanynorth",
		"Norway West":              "norwaywest",
		"Switzerland West":         "switzerlandwest",
		"UK West":                  "ukwest",
		"UAE Central":              "uaecentral",
		"Brazil Southeast":         "brazilsoutheast",

		// Azure Government locations.
		//
		// https://learn.microsoft.com/en-us/azure/azure-government/documentation-government-get-started-connect-with-ps
		"USDoD Central":      "usdodcentral",
		"USDoD East":         "usdodeast",
		"USGov Arizona":      "usgovarizona",
		"USGov Iowa":         "usgoviowa",
		"USGov Texas":        "usgovtexas",
		"USGov Virginia":     "usgovvirginia",
		"USSec East":         "usseceast",
		"USSec West":         "ussecwest",
		"USSec West Central": "ussecwestcentral",

		// Azure China locations.
		"China East":    "chinaeast",
		"China East 2":  "chinaeast2",
		"China North":   "chinanorth",
		"China North 2": "chinanorth2",
		"China North 3": "chinanorth3",
	}
	// locationsToDisplayNames maps Azure location names to their display
	// names. This is the reverse lookup map of displayNamesToLocations.
	locationsToDisplayNames = map[string]string{}
)

func initLocations() {
	for displayName, location := range displayNamesToLocations {
		locationsToDisplayNames[location] = displayName
	}
}

func init() {
	initLocations()
}
