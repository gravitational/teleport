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

package auth

// Trust is responsible for managing certificate authorities
// Each authority is managing some domain, e.g. example.com
//
// There are two type of authorities, local and remote.
// Local authorities have both private and public keys, so they can
// sign public keys of users and hosts
//
// Remote authorities have only public keys available, so they can
// be only used to validate
type Trust interface {
	// UpsertCertAuthority updates or inserts a new certificate authority
	UpsertCertAuthority(ca CertAuthority) error

	// CompareAndSwapCertAuthority updates the cert authority value
	// if existing value matches existing parameter,
	// returns nil if succeeds, trace.CompareFailed otherwise
	CompareAndSwapCertAuthority(new, existing CertAuthority) error

	// DeleteCertAuthority deletes particular certificate authority
	DeleteCertAuthority(id CertAuthID) error

	// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
	// controls if signing keys are loaded
	GetCertAuthority(id CertAuthID, loadSigningKeys bool, opts ...MarshalOption) (CertAuthority, error)

	// GetCertAuthorities returns a list of authorities of a given type
	// loadSigningKeys controls whether signing keys should be loaded or not
	GetCertAuthorities(caType CertAuthType, loadSigningKeys bool, opts ...MarshalOption) ([]CertAuthority, error)
}
