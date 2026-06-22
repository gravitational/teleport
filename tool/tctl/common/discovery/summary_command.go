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
}

func (s *summaryArgs) initSummary(app *kingpin.CmdClause) {
	summaryCmd := app.Command("summary", "Summarize AWS and Azure discovery_config resources and their enrollment progress.")

	summaryCmd.Flag("format", "Output format.").
		Default(teleport.Text).
		EnumVar(&s.format, teleport.Text, teleport.JSON, teleport.YAML)
	summaryCmd.Flag("cloud", "Comma-separated list of cloud providers to include (allowed: aws, azure). Empty (default) returns all.").
		Default("").
		StringVar(&s.cloudFilter)

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
	summaries := buildConfigSummaries(discoveryConfigs, cloudProviders)
	switch s.format {
	case teleport.Text:
		return trace.Wrap(renderSummaryText(w, summaries, time.Now()))
	case teleport.JSON:
		structuredSummaries := buildStructuredSummaries(summaries)
		return trace.Wrap(utils.WriteJSONArray(w, structuredSummaries))
	case teleport.YAML:
		structuredSummaries := buildStructuredSummaries(summaries)
		return trace.Wrap(utils.WriteYAML(w, structuredSummaries))
	default:
		return trace.BadParameter("unknown format %q", s.format)
	}
}
