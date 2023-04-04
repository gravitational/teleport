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

package utils

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/exp/slices"
)

func isFQDN(hostname string) bool {
	return strings.Contains(hostname, ".")
}

func isWildcard(hostname string) bool {
	return strings.HasPrefix(hostname, "*.")
}

func wildcardFromHostname(hostname string) string {
	if isWildcard(hostname) {
		return hostname
	}
	return fmt.Sprintf("*.%s", strings.Join(strings.Split(hostname, ".")[1:], "."))
}

// AddHostToHostList adds the provided hostname to the hostList, then processes the hostList to add wildcards
// if appropriate. it returns a processed, sorted hostList.
//
// for every host in the list:
//   - all hostnames are converted to their wildcard form (test.example.com -> *.example.com) and grouped together.
//   - if we only have a single hostname which would be matched by a given wildcard, that hostname is added
//     to the hostList verbatim to prevent matching against too many hosts with a wildcard.
//   - if a wildcard matches multiple hostnames, the wildcard will be added to the hostList instead.
//
// this is an effort to keep the length of hostList as short as possible for efficiency and tidiness.
func AddHostToHostList(hostList []string, hostname string) []string {
	// add the incoming hostname to the hostList before we sort and process it
	hostList = append(hostList, hostname)

	hostMap := make(map[string][]string)
	// iterate over the full hostList
	// if the element is a wildcard, add it to the list of wildcards
	// if the element is not a wildcard, convert it to a wildcard and add any hostnames it matches to a map
	for _, element := range hostList {
		// FQDN-based hosts are grouped under a wildcard key
		if isFQDN(element) {
			wildcard := wildcardFromHostname(element)
			if !slices.Contains(hostMap[wildcard], element) {
				hostMap[wildcard] = append(hostMap[wildcard], element)
			}
		} else {
			// any non-wildcard hosts go into the "extra" key and will be processed separately at the end
			hostMap["extra"] = append(hostMap["extra"], element)
		}
	}

	// iterate over the map, look for all wildcard keys with more than one hostname matching.
	// for each match, add the wildcard to the hostList.
	var outputHostList []string
	for key, matchingHostnames := range hostMap {
		// add all non-wildcard matches separately
		if key == "extra" {
			for _, hostname := range matchingHostnames {
				if !slices.Contains(outputHostList, hostname) {
					outputHostList = append(outputHostList, hostname)
				}
			}
		} else {
			// add all wildcards with more than one hostname matching
			if len(matchingHostnames) > 1 {
				outputHostList = append(outputHostList, key)
			} else {
				// add the single hostname which is matched by the given wildcard
				outputHostList = append(outputHostList, matchingHostnames[0])
			}
		}
	}

	slices.Sort(outputHostList)
	return outputHostList
}

// NaivelyValidateHostname checks the provided hostname against a naive regex to ensure it doesn't contain obviously
// illegal characters. It's not guaranteed to be perfect, just a simple sanity check.
func NaivelyValidateHostname(hostname string) bool {
	hostnameRegexp := regexp.MustCompile("^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]).)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9-]*[A-Za-z0-9])$")
	return hostnameRegexp.Match([]byte(hostname))
}
