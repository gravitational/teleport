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
	"context"
	"io"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/utils"
)

type statusArgs struct {
	cmd *kingpin.CmdClause

	format      string
	cloudFilter string
}

func (s *statusArgs) initStatus(app *kingpin.CmdClause) {
	statusCmd := app.Command("status", "Show auto-discovery status and enrollment progress.")

	statusCmd.Flag("format", "Output format.").
		Default(teleport.Text).
		EnumVar(&s.format, teleport.Text, teleport.JSON, teleport.YAML)
	statusCmd.Flag("cloud", "Comma-separated list of cloud providers to include (allowed: aws, azure). Empty (default) returns all.").
		Default("").
		StringVar(&s.cloudFilter)

	s.cmd = statusCmd
}

func (s *statusArgs) run(ctx context.Context, clt discoveryClient, w io.Writer) error {
	cloudProviders, err := parseCloudProviders(s.cloudFilter)
	if err != nil {
		return trace.Wrap(err)
	}

	discoveryConfigs, err := stream.Collect(clientutils.Resources(ctx, clt.DiscoveryConfigClient().ListDiscoveryConfigs))
	if err != nil {
		return trace.Wrap(err)
	}
	status := newDiscoverySummary(discoveryConfigs, cloudProviders)
	switch s.format {
	case teleport.Text:
		return trace.Wrap(status.renderText(w, time.Now()))
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSONArray(w, status))
	case teleport.YAML:
		return trace.Wrap(utils.WriteYAML(w, status))
	default:
		return trace.BadParameter("unknown format %q", s.format)
	}
}
