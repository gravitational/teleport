/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

package recordingmetadatav1

import (
	"testing"
	"time"

	"pgregory.net/rapid"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/utils/testutils"
)

func TestProperty_TTYThumbnail_NeverPanicsOnRandomEventSequence(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(0, 30).Draw(t, "count")
		events := make([]apievents.AuditEvent, count)
		for i := range events {
			events[i] = genEvent(t, "evt")
		}

		testutils.RunWithTimeout(t, 10*time.Second, func() {
			gen := newTTYThumbnailGenerator()
			defer gen.release()

			for _, evt := range events {
				_ = gen.handleEvent(evt)
			}
			_, _ = gen.produceThumbnail(0)
		})
	})
}

var extremeResizeSizes = []string{
	"0:0", "1:0", "0:1", "1:1", "2:2",
	"1000:500", "2000:1000",
	"2048:2048", "2049:2049",
	"99999:99999", "9999999:9999999",
}

func TestTTYThumbnail_NeverPanicsOnZeroOrExtremeResize(t *testing.T) {
	for _, size := range extremeResizeSizes {
		t.Run(size, func(t *testing.T) {
			testutils.RunWithTimeout(t, 10*time.Second, func() {
				gen := newTTYThumbnailGenerator()
				defer gen.release()

				_ = gen.handleEvent(&apievents.SessionStart{TerminalSize: size})
				_ = gen.handleEvent(&apievents.SessionPrint{Data: []byte("hello world\r\n")})
				_, _ = gen.produceThumbnail(0)
			})
		})
	}
}

func TestProperty_TTYThumbnail_NeverPanicsBeforeSessionStart(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// SessionPrint and Resize delivered without prior SessionStart.
		data := rapid.SliceOfN(rapid.Byte(), 0, 512).Draw(t, "data")

		testutils.RunWithTimeout(t, 10*time.Second, func() {
			gen := newTTYThumbnailGenerator()
			defer gen.release()

			_ = gen.handleEvent(&apievents.SessionPrint{Data: data})
			_ = gen.handleEvent(&apievents.Resize{TerminalSize: "80:24"})
			_, _ = gen.produceThumbnail(0)
		})
	})
}

var ttyEventKinds = []string{
	"start", "resize", "print", "print_ansi", "print_long", "other",
}

// genEvent produces a random TTY event. Bias toward control sequences in SessionPrint so the vt10x parser exercises
// escape-code paths.
func genEvent(t *rapid.T, label string) apievents.AuditEvent {
	t.Helper()

	kind := rapid.SampledFrom(ttyEventKinds).Draw(t, label+"_kind")

	switch kind {
	case "start":
		return &apievents.SessionStart{TerminalSize: genTerminalSize(t, label+"_size")}

	case "resize":
		return &apievents.Resize{TerminalSize: genTerminalSize(t, label+"_size")}

	case "print":
		return &apievents.SessionPrint{
			Data: rapid.SliceOfN(rapid.Byte(), 0, 128).Draw(t, label+"_data"),
		}

	case "print_ansi":
		return &apievents.SessionPrint{
			Data: rapid.SliceOfN(
				rapid.OneOf(
					rapid.Just(byte(0x1b)),
					rapid.Just(byte('[')),
					rapid.Just(byte(']')),
					rapid.Just(byte('?')),
					rapid.Just(byte('h')),
					rapid.Just(byte('l')),
					rapid.Byte(),
				),
				0, 128,
			).Draw(t, label+"_ansi"),
		}

	case "print_long":
		return &apievents.SessionPrint{
			Data: rapid.SliceOfN(rapid.Byte(), 256, 1024).Draw(t, label+"_long"),
		}

	default:
		return &apievents.SessionEnd{}
	}
}

// genTerminalSize produces "W:H" strings biased toward edge cases, including values exceeding the vt10x resize cap
// (2048 per dimension), which the terminal silently ignores. UnmarshalTerminalParams itself imposes no range limit.
func genTerminalSize(t *rapid.T, label string) string {
	t.Helper()

	return rapid.OneOf(
		rapid.Just(""),
		rapid.Just("0:0"),
		rapid.Just("1:1"),
		rapid.Just("80:24"),
		rapid.Just("1000:500"),
		rapid.Just("99999:99999"),
		rapid.Just("not-a-size"),
		rapid.StringN(0, 32, -1),
	).Draw(t, label)
}
