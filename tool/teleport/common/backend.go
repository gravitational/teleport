// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package common

import (
	"context"
	"os"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/backend"
)

func onClone(ctx context.Context, configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return trace.Wrap(err)
	}
	var config backend.CloneConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return trace.Wrap(err)
	}

	src, err := backend.New(ctx, config.Source.Type, config.Source.Params)
	if err != nil {
		return trace.Wrap(err, "failed to create source backend")
	}
	defer src.Close()

	dst, err := backend.New(ctx, config.Destination.Type, config.Destination.Params)
	if err != nil {
		return trace.Wrap(err, "failed to create destination backend")
	}
	defer dst.Close()

	if err := backend.Clone(ctx, src, dst, config.Parallel, config.Force); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
