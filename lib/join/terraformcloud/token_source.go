/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package terraformcloud

import (
	"fmt"
	"strings"

	"github.com/gravitational/trace"
)

type envGetter func(key string) string

// IDTokenSource allows a Terraform Workload ID token to be fetched whilst
// within a Terraform Cloud execution.
type IDTokenSource struct {
	audienceTag string

	getEnv envGetter
}

// GetIDToken fetches a Terraform Cloud JWT from the local node's environment
func (its *IDTokenSource) GetIDToken() (string, error) {
	name := "TFC_WORKLOAD_IDENTITY_TOKEN"
	if its.audienceTag != "" {
		name = fmt.Sprintf("TFC_WORKLOAD_IDENTITY_TOKEN_%s", strings.ToUpper(its.audienceTag))
	}

	tok := its.getEnv(name)
	if tok == "" {
		audienceName := "TFC_WORKLOAD_IDENTITY_AUDIENCE"
		if its.audienceTag != "" {
			audienceName = fmt.Sprintf("TFC_WORKLOAD_IDENTITY_AUDIENCE_%s", strings.ToUpper(its.audienceTag))
		}

		return "", trace.BadParameter(
			"%s environment variable missing, ensure the corresponding %s variable is set in the workspace",
			name, audienceName,
		)
	}

	return tok, nil
}

// NewIDTokenSource creates a new TFC ID token source with the given audience
// tag.
func NewIDTokenSource(audienceTag string, getEnv envGetter) *IDTokenSource {
	return &IDTokenSource{
		audienceTag,
		getEnv,
	}
}
