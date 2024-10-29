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

package handler

import (
	"github.com/gravitational/trace"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/daemon"
	"github.com/gravitational/teleport/lib/ui"
)

// New creates an instance of Handler
func New(cfg Config) (*Handler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Handler{
		Config: cfg,
	}, nil
}

// Config is the terminal service configuration
type Config struct {
	// DaemonService is the instance of daemon service
	DaemonService *daemon.Service
}

// CheckAndSetDefaults checks and sets the defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.DaemonService == nil {
		return trace.BadParameter("missing DaemonService")
	}

	return nil
}

// Handler implements teleterm api service
type Handler struct {
	api.UnimplementedTerminalServiceServer

	// Config is the service config
	Config
}

func makeAPILabels(uiLabels []ui.Label) []*api.Label {
	apiLabels := make([]*api.Label, 0, len(uiLabels))
	for _, uiLabel := range uiLabels {
		apiLabels = append(apiLabels, &api.Label{
			Name:  uiLabel.Name,
			Value: uiLabel.Value,
		})
	}
	return apiLabels
}
