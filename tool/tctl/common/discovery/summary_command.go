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
	"github.com/gravitational/teleport/lib/utils"
)

type summaryArgs struct {
	cmd *kingpin.CmdClause

	format      string
	cloudFilter string
	integration string
}

func (s *summaryArgs) initSummary(app *kingpin.CmdClause) {
	summaryCmd := app.Command("summary", "") //todo (mpm) get summary help message

	summaryCmd.Flag("format", "Output format.").
		Default(teleport.Text).
		EnumVar(&s.format, teleport.Text, teleport.JSON, teleport.YAML)
	summaryCmd.Flag("cloud", "Comma-separated list of cloud providers to include (allowed: aws, azure). Empty (default) returns all.").
		Default("").
		StringVar(&s.cloudFilter)
	summaryCmd.Flag("integration", "").
		Default("").
		StringVar(&s.integration) //todo (mpm) get integration help message

	s.cmd = summaryCmd
}

func (s *summaryArgs) run(ctx context.Context, clt discoveryClient, w io.Writer) error {
	cloudProviders, err := parseCloudProviders(s.cloudFilter)
	if err != nil {
		return trace.Wrap(err)
	}

	discoveryConfigs, err := listDiscoveryConfigs(ctx, clt)
	if err != nil {
		return trace.Wrap(err)
	}
	switch s.format {
	case teleport.Text:
		blocks := buildSummaryBlocks(discoveryConfigs, cloudProviders, s.integration)
		return trace.Wrap(renderSummaryText(w, blocks, time.Now()))
	case teleport.JSON:
		summaries := buildSummaries(discoveryConfigs, cloudProviders, s.integration)
		return trace.Wrap(utils.WriteJSONArray(w, summaries))
	case teleport.YAML:
		summaries := buildSummaries(discoveryConfigs, cloudProviders, s.integration)
		return trace.Wrap(utils.WriteYAML(w, summaries))
	default:
		return trace.BadParameter("unknown format %q", s.format)
	}
}
