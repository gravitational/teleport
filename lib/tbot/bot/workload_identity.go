/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package bot

import (
	"github.com/gravitational/trace"

	apiutils "github.com/gravitational/teleport/api/utils"
)

// WorkloadIdentitySelector allows the user to select which WorkloadIdentity
// resource should be used.
//
// Only one of Name or Labels can be set.
type WorkloadIdentitySelector struct {
	// Name is the name of a specific WorkloadIdentity resource.
	Name string `yaml:"name"`
	// Labels is a set of labels that the WorkloadIdentity resource must have.
	Labels map[string][]string `yaml:"labels,omitempty"`
}

// CheckAndSetDefaults checks the WorkloadIdentitySelector values and sets any
// defaults.
func (s *WorkloadIdentitySelector) CheckAndSetDefaults() error {
	switch {
	case s.Name == "" && len(s.Labels) == 0:
		return trace.BadParameter("one of ['name', 'labels'] must be set")
	case s.Name != "" && len(s.Labels) > 0:
		return trace.BadParameter("at most one of ['name', 'labels'] can be set")
	}
	for k, v := range s.Labels {
		if len(v) == 0 {
			return trace.BadParameter("labels[%s]: must have at least one value", k)
		}
	}
	return nil
}

// TrustDomain identifies a Teleport-managed trust domain that workloads can
// opt in to via a TrustDomainsSelector.
type TrustDomain string

const (
	// TrustDomainAppClient is the trust domain used to validate certificates
	// issued by the Teleport application service.
	TrustDomainAppClient TrustDomain = "app_client"
)

// TrustDomainsSelector selects additional Teleport-managed trust domains
// whose bundles should be included alongside the workload identity trust domain.
type TrustDomainsSelector []TrustDomain

// CheckAndSetDefaults checks the TrustDomainsSelector values and sets any
// defaults.
func (tds *TrustDomainsSelector) CheckAndSetDefaults() error {
	*tds = apiutils.Deduplicate(*tds)
	for _, domain := range *tds {
		switch domain {
		case TrustDomainAppClient:
		default:
			return trace.BadParameter("invalid trust domain %q. supported trust_domains: %q", domain, TrustDomainAppClient)
		}
	}

	return nil
}
