/*
Copyright 2021 Gravitational, Inc.

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

package services

// These are all deprecated functions that will be removed in a follow up PR,
// as soon as their references are updated in the e submodule.

type Marshaler struct{}

func GetRoleMarshaler() Marshaler {
	return Marshaler{}
}

func (Marshaler) MarshalRole(r Role, opts ...MarshalOption) ([]byte, error) {
	return MarshalRole(r, opts...)
}

func (Marshaler) UnmarshalRole(bytes []byte, opts ...MarshalOption) (Role, error) {
	return UnmarshalRole(bytes, opts...)
}

func GetOIDCConnectorMarshaler() Marshaler {
	return Marshaler{}
}

func (Marshaler) MarshalOIDCConnector(r OIDCConnector, opts ...MarshalOption) ([]byte, error) {
	return MarshalOIDCConnector(r, opts...)
}

func (Marshaler) UnmarshalOIDCConnector(bytes []byte, opts ...MarshalOption) (OIDCConnector, error) {
	return UnmarshalOIDCConnector(bytes, opts...)
}

func GetSAMLConnectorMarshaler() Marshaler {
	return Marshaler{}
}

func (Marshaler) MarshalSAMLConnector(r SAMLConnector, opts ...MarshalOption) ([]byte, error) {
	return MarshalSAMLConnector(r, opts...)
}

func (Marshaler) UnmarshalSAMLConnector(bytes []byte, opts ...MarshalOption) (SAMLConnector, error) {
	return UnmarshalSAMLConnector(bytes, opts...)
}

type GithubMarshaler struct{}

func GetGithubConnectorMarshaler() GithubMarshaler {
	return GithubMarshaler{}
}

func (GithubMarshaler) Unmarshal(bytes []byte, opts ...MarshalOption) (GithubConnector, error) {
	return UnmarshalGithubConnector(bytes)
}

type TrustedClusterMarshaler struct{}

func GetTrustedClusterMarshaler() TrustedClusterMarshaler {
	return TrustedClusterMarshaler{}
}

func (TrustedClusterMarshaler) Unmarshal(bytes []byte, opts ...MarshalOption) (TrustedCluster, error) {
	return UnmarshalTrustedCluster(bytes)
}
