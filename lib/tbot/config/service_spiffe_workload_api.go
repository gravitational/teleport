/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package config

import (
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
)

const SPIFFEWorkloadAPIServiceType = "spiffe-workload-api"

// SPIFFEWorkloadAPIService is the configuration for the SPIFFE Workload API
// service.
type SPIFFEWorkloadAPIService struct {
	// Listen is the address on which the SPIFFE Workload API server should
	// listen. This should either be prefixed with "unix://" or "tcp://".
	Listen string `yaml:"listen"`
	// SVIDs is the list of SVIDs that the SPIFFE Workload API server should
	// provide.
	SVIDs []SVIDRequest `yaml:"svids"`
}

func (s *SPIFFEWorkloadAPIService) Type() string {
	return SPIFFEWorkloadAPIServiceType
}

func (s *SPIFFEWorkloadAPIService) MarshalYAML() (interface{}, error) {
	type raw SPIFFEWorkloadAPIService
	return withTypeHeader((*raw)(s), SPIFFEWorkloadAPIServiceType)
}

func (s *SPIFFEWorkloadAPIService) UnmarshalYAML(node *yaml.Node) error {
	// Alias type to remove UnmarshalYAML to avoid recursion
	type raw SPIFFEWorkloadAPIService
	if err := node.Decode((*raw)(s)); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *SPIFFEWorkloadAPIService) CheckAndSetDefaults() error {
	if s.Listen == "" {
		return trace.BadParameter("listen: should not be empty")
	}
	if len(s.SVIDs) == 0 {
		return trace.BadParameter("svids: should not be empty")
	}
	for i, svid := range s.SVIDs {
		if err := svid.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err, "validiting svid[%d]", i)
		}
	}
	return nil
}
