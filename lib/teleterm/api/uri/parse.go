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

package uri

import "github.com/gravitational/trace"

type validateFunc func(ResourceURI) error

// Parse parses provided path as a cluster URI or a cluster resource URI.
func Parse(path string, validateFuncs ...validateFunc) (ResourceURI, error) {
	r := New(path)

	for _, validate := range append(
		[]validateFunc{checkProfileName}, // Basic validation.
		validateFuncs...,                 // Extra validations.
	) {
		if err := validate(r); err != nil {
			return ResourceURI{}, trace.Wrap(err)
		}
	}
	return r, nil
}

// ParseGatewayTargetURI parses the provided path as a gateway target URI.
func ParseGatewayTargetURI(path string) (ResourceURI, error) {
	r, err := Parse(path, checkGatewayTargetResource)
	return r, trace.Wrap(err)
}

func checkProfileName(r ResourceURI) error {
	if r.GetProfileName() == "" {
		return trace.BadParameter("missing root cluster name")
	}
	return nil
}

func checkGatewayTargetResource(r ResourceURI) error {
	if r.GetDbName() == "" && r.GetKubeName() == "" {
		return trace.BadParameter("missing target resource name")
	}
	return nil
}
