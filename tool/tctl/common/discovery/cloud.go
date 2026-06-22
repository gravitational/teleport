// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package discovery

import (
	"strings"

	"github.com/gravitational/trace"
)

// Cloud provider display names. These double as the Cloud field value in
// summary output and as the discriminator for cloud-specific behavior.
const (
	cloudAWS   = "AWS"
	cloudAzure = "Azure"
)

// cloudProviderConfig records which cloud providers a command should include.
type cloudProviderConfig struct {
	aws, azure bool
}

// parseCloudProviders parses the --cloud flag. An empty value includes every
// supported provider.
func parseCloudProviders(value string) (cloudProviderConfig, error) {
	const (
		cloudProviderAWS   = "aws"
		cloudProviderAzure = "azure"
	)

	if value == "" {
		return cloudProviderConfig{
			aws:   true,
			azure: true,
		}, nil
	}

	cfg := cloudProviderConfig{}
	parts := strings.Split(value, ",")
	for _, part := range parts {
		switch strings.ToLower(strings.TrimSpace(part)) {
		case cloudProviderAWS:
			cfg.aws = true
		case cloudProviderAzure:
			cfg.azure = true
		case "":
			return cloudProviderConfig{}, trace.BadParameter("empty cloud provider in --cloud (allowed: aws, azure)")
		default:
			return cloudProviderConfig{}, trace.BadParameter("unknown cloud provider %q (allowed: aws, azure)", part)
		}
	}
	return cfg, nil
}
