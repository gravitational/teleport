/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package recordings

import (
	"strings"
	"testing"

	summarizerv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
)

func TestRenderTimelineSanitizesTerminalControlSequences(t *testing.T) {
	timeline := renderTimeline(summarizerv1pb.EnhancedSummary_builder{
		NotableCommandIndexes: []int32{0},
		Commands: []*summarizerv1pb.CommandAnalysis{
			summarizerv1pb.CommandAnalysis_builder{
				TimelineTitle:    "\x1b]0;owned title\aListed files\x1b[31m",
				TimelineSubtitle: "\x1b]8;;https://example.com\aDenied\x1b]8;;\a",
				RiskLevel:        summarizerv1pb.RiskLevel_RISK_LEVEL_HIGH,
			}.Build(),
			summarizerv1pb.CommandAnalysis_builder{
				Command: "\x1b]52;c;AAAA\acat /etc/passwd",
			}.Build(),
		},
	}.Build(), 100, buildPalette())

	visible := stripANSI(timeline)
	for _, forbidden := range []string{"\x1b", "\a", "owned title", "AAAA"} {
		if strings.Contains(visible, forbidden) {
			t.Fatalf("renderTimeline output contains %q: %q", forbidden, visible)
		}
	}
	for _, want := range []string{"Listed files", "Denied", "cat /etc/passwd"} {
		if !strings.Contains(visible, want) {
			t.Fatalf("renderTimeline output = %q, want to contain %q", visible, want)
		}
	}
}
