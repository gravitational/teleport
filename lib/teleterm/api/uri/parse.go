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

// ValidateFunc validates the provided ResourceURI.
type ValidateFunc func(ResourceURI) error

// Parse parses provided path as a cluster URI or a cluster resource URI.
func Parse(path string, validateFuncs ...ValidateFunc) (ResourceURI, error) {
	r := New(path)

	for _, validate := range append(
		[]ValidateFunc{validateProfileName}, // Basic validation.
		validateFuncs...,                    // Extra validations.
	) {
		if err := validate(r); err != nil {
			return ResourceURI{}, trace.Wrap(err)
		}
	}
	return r, nil
}

// ParseGatewayTargetURI parses the provided path as a gateway target URI.
func ParseGatewayTargetURI(path string) (ResourceURI, error) {
	r, err := Parse(path, validateGatewayTargetResource)
	return r, trace.Wrap(err)
}

// ParseDBURI parses the provided path as a database URI.
func ParseDBURI(path string) (ResourceURI, error) {
	r, err := Parse(path, validateDBResource)
	return r, trace.Wrap(err)
}

func validateProfileName(r ResourceURI) error {
	if r.GetProfileName() == "" {
		return trace.BadParameter("malformed URI %q, missing profile name", r)
	}
	return nil
}

func validateGatewayTargetResource(r ResourceURI) error {
	if r.GetDbName() == "" && r.GetKubeName() == "" {
		return trace.BadParameter("malformed gateway target URI %q, expecting a database or kube resource", r)
	}
	return nil
}

func validateDBResource(r ResourceURI) error {
	if r.GetDbName() == "" {
		return trace.BadParameter("malformed database URI %q, missing database name", r)
	}
	return nil
}
