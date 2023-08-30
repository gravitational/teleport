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

package aws

import (
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/service/sts"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

var (
	globalSTSEndpoints = []string{
		"sts.amazonaws.com",
		// This is not a real endpoint, but the SDK will select it if
		// AWS_USE_FIPS_ENDPOINT is set and a region is not.
		"sts-fips.aws-global.amazonaws.com",
	}

	fipsSTSEndpoints     []string
	validSTSEndpoints    []string
	initSTSEndpointsOnce sync.Once
)

// GetSTSGlobalEndpoints returns a list of global STS endpoints.
func GetSTSGlobalEndpoints() []string {
	return globalSTSEndpoints
}

// GetSTSFipsEndpoints returns a list of STS fips endpoints.
func GetSTSFipsEndpoints() []string {
	initSTSEndpoints()
	return fipsSTSEndpoints
}

// GetValidSTSEndpoints returns a list of all valid STS endpoints.
func GetValidSTSEndpoints() []string {
	initSTSEndpoints()
	return validSTSEndpoints
}

func initSTSEndpoints() {
	initSTSEndpointsOnce.Do(func() {
		nullOption := func(*endpoints.Options) {}

		fipsSTSEndpoints = genSTSEndpoints(genSTSEndointConfig{
			requiredOptions: []func(*endpoints.Options){
				endpoints.StrictMatchingOption,
				endpoints.UseFIPSEndpointOption,
			},
			multiplyOptions: []func(*endpoints.Options){
				nullOption,
				endpoints.STSRegionalEndpointOption,
				endpoints.UseDualStackEndpointOption,
			},
		})

		validSTSEndpoints = genSTSEndpoints(genSTSEndointConfig{
			requiredOptions: []func(*endpoints.Options){
				endpoints.StrictMatchingOption,
			},
			multiplyOptions: []func(*endpoints.Options){
				nullOption,
				endpoints.STSRegionalEndpointOption,
				endpoints.UseFIPSEndpointOption,
				endpoints.UseDualStackEndpointOption,
			},
		})
	})
}

// combinations returns all combinations for a given array. This is essentially
// a powerset of the given set except that the empty set is disregarded.
//
// Reference: https://github.com/mxschmitt/golang-combinations
func combinations[T any](set []T) (subsets [][]T) {
	length := uint(len(set))

	// Go through all possible combinations of objects
	// from 1 (only first object in subset) to 2^length (all objects in subset)
	for subsetBits := 1; subsetBits < (1 << length); subsetBits++ {
		var subset []T

		for object := uint(0); object < length; object++ {
			// checks if object is contained in subset
			// by checking if bit 'object' is set in subsetBits
			if (subsetBits>>object)&1 == 1 {
				// add object to subset
				subset = append(subset, set[object])
			}
		}
		// add subset to subsets
		subsets = append(subsets, subset)
	}
	return subsets
}

type genSTSEndointConfig struct {
	// requiredOptions are endpoints options that must be set for each
	// EndpointFor call.
	requiredOptions []func(*endpoints.Options)
	// multiplyOptions is a list of endpoints options where all their
	// combinations must be iterated to create the endpoints.
	multiplyOptions []func(*endpoints.Options)
}

func genSTSEndpoints(cfg genSTSEndointConfig) []string {
	optCombinations := combinations(cfg.multiplyOptions)
	endpointsSet := make(map[string]struct{})
	for _, partition := range endpoints.DefaultPartitions() {
		for region := range partition.Regions() {
			for _, opts := range optCombinations {
				endpoint, err := partition.EndpointFor(sts.ServiceName, region, append(cfg.requiredOptions, opts...)...)
				if err != nil {
					// Skip if no endpoint found for this opts combo.
					continue
				}

				endpointsSet[strings.TrimPrefix(endpoint.URL, "https://")] = struct{}{}
			}
		}
	}

	endpointsSlice := maps.Keys(endpointsSet)
	slices.Sort(endpointsSlice)
	return endpointsSlice
}
