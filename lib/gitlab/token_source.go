/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package gitlab

import (
	"os"

	"github.com/gravitational/trace"
)

type envGetter func(key string) string

// IDTokenSource allows a GitLab ID token to be fetched whilst executing
// within the context of a GitLab actions workflow.
type IDTokenSource struct {
	config IDTokenSourceConfig
}

func (its *IDTokenSource) GetIDToken() (string, error) {
	tok := its.config.EnvGetter(its.config.EnvVarName)
	if tok == "" {
		return "", trace.BadParameter(
			"%q environment variable missing", its.config.EnvVarName,
		)
	}

	return tok, nil
}

type IDTokenSourceConfig struct {
	// EnvGetter provides a custom function for fetching the environment
	// variables. This is useful for testing purposes. If unset, this will
	// default to os.Getenv.
	EnvGetter envGetter
	// EnvVarName is the name of the environment variable that contains the
	// IDToken. If unset, this will default to "TBOT_GITLAB_JWT".
	EnvVarName string
}

func NewIDTokenSource(config IDTokenSourceConfig) *IDTokenSource {
	if config.EnvGetter == nil {
		config.EnvGetter = os.Getenv
	}
	if config.EnvVarName == "" {
		config.EnvVarName = "TBOT_GITLAB_JWT"
	}
	return &IDTokenSource{
		config: config,
	}
}
