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

func TestSanitizeRemovesTerminalControlSequences(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "csi",
			in:   "safe\x1b[31mred\x1b[0m text",
			want: "safered text",
		},
		{
			name: "osc_bel",
			in:   "safe\x1b]0;owned title\a text",
			want: "safe text",
		},
		{
			name: "osc_st",
			in:   "safe\x1b]8;;https://example.com\x1b\\link\x1b]8;;\x1b\\ text",
			want: "safelink text",
		},
		{
			name: "unterminated_osc",
			in:   "safe\x1b]0;owned title",
			want: "safe",
		},
		{
			name: "raw_c1_csi",
			in:   "safe\x9b31mred text",
			want: "safered text",
		},
		{
			name: "utf8_preserved",
			in:   "safe caf\xc3\xa9\n\ttext",
			want: "safe caf\xc3\xa9\n\ttext",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitize(tt.in); got != tt.want {
				t.Fatalf("sanitize(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRenderTimelineSanitizesTerminalControlSequences(t *testing.T) {
	timeline := renderTimeline(&summarizerv1pb.EnhancedSummary{
		NotableCommandIndexes: []int32{0},
		Commands: []*summarizerv1pb.CommandAnalysis{
			{
				TimelineTitle:    "\x1b]0;owned title\aListed files\x1b[31m",
				TimelineSubtitle: "\x1b]8;;https://example.com\aDenied\x1b]8;;\a",
				RiskLevel:        summarizerv1pb.RiskLevel_RISK_LEVEL_HIGH,
			},
			{
				Command: "\x1b]52;c;AAAA\acat /etc/passwd",
			},
		},
	}, 100, buildPalette())

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
