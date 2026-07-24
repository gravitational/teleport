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

package servicecfg

import (
	"fmt"
	"net/netip"
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestProperty_ParseTargetHostPrefixes_RejectsHostileEntries asserts that the
// target host parser is a complete filter: any entry carrying a disqualifying
// attribute is rejected as a BadParameter, including combinations of attributes,
// and a single such entry rejects the whole list regardless of its position.
// The parser is a security control, so the guarantee it must uphold is negative
// (nothing hostile is ever accepted), and no combination of otherwise-rejected
// forms may cancel out into an accepted one.
func TestProperty_ParseTargetHostPrefixes_RejectsHostileEntries(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		bad := genHostileEntry(t)

		_, err := ParseTargetHostPrefixes([]string{bad})
		require.True(t, trace.IsBadParameter(err), "want BadParameter for hostile entry %q, got %v", bad, err)

		// The valid entries alone must parse: this both exercises the accepting
		// path and guards the test against a generator that produces something
		// unexpectedly invalid, which would make the mixed-list check vacuous.
		valid := genValidEntries(t)
		_, err = ParseTargetHostPrefixes(valid)
		require.NoError(t, err, "valid entries unexpectedly rejected: %v", valid)

		idx := rapid.IntRange(0, len(valid)).Draw(t, "insert_idx")
		mixed := slices.Insert(slices.Clone(valid), idx, bad)
		_, err = ParseTargetHostPrefixes(mixed)
		require.True(t, trace.IsBadParameter(err),
			"want BadParameter for list with hostile %q at index %d: %v", bad, idx, mixed)
	})
}

// genHostileEntry draws a string that ParseTargetHostPrefixes must reject. Each
// branch is self-contained and provably disqualifying, several of them stacking
// multiple hostile attributes (zone, IPv4-in-IPv6, non-canonical CIDR) that the
// parser must reject no matter how they combine.
func genHostileEntry(t *rapid.T) string {
	zone := rapid.SampledFrom([]string{"eth0", "en0", "1"}).Draw(t, "zone")
	return rapid.OneOf(
		// Zone-tagged address, and zone-tagged CIDR (zone + CIDR combined).
		rapid.Custom(func(t *rapid.T) string {
			addr := genCleanAddr(t)
			if rapid.Bool().Draw(t, "zoned_cidr") {
				bits := rapid.IntRange(0, addr.BitLen()).Draw(t, "zone_bits")
				return fmt.Sprintf("%s%%%s/%d", addr, zone, bits)
			}
			return fmt.Sprintf("%s%%%s", addr, zone)
		}),
		// IPv4-in-IPv6: bare, zone-tagged, and as a CIDR (mapped + zone/CIDR).
		rapid.Custom(func(t *rapid.T) string {
			mapped := netip.AddrFrom16(genV4(t).As16())
			switch rapid.IntRange(0, 2).Draw(t, "mapped_shape") {
			case 0:
				return mapped.String()
			case 1:
				return mapped.WithZone(zone).String()
			default:
				bits := rapid.IntRange(0, 128).Draw(t, "mapped_bits")
				return fmt.Sprintf("%s/%d", mapped, bits)
			}
		}),
		// Non-canonical CIDR: a prefix with a host bit set.
		rapid.Custom(genNonCanonicalCIDR),
		// A hostname, which is neither an IP nor a CIDR.
		rapid.StringMatching(`[a-z][a-z-]{0,9}(\.[a-z][a-z-]{0,9}){0,3}`),
		// Empty and whitespace-only entries.
		rapid.SampledFrom([]string{"", " ", "\t", "  \n "}),
	).Draw(t, "hostile_entry")
}

// genNonCanonicalCIDR builds a CIDR string whose address has a host bit set, so
// its masked form differs from what was written. A random bit from anywhere in
// the host portion is set (not always the lowest), so a validator that inspects
// only one position cannot slip through.
func genNonCanonicalCIDR(t *rapid.T) string {
	base := genCleanAddr(t)
	// Leave at least one host bit so setting one makes the prefix non-canonical.
	bits := rapid.IntRange(0, base.BitLen()-1).Draw(t, "noncanon_bits")
	hostBit := rapid.IntRange(bits, base.BitLen()-1).Draw(t, "host_bit_pos")
	network := netip.PrefixFrom(base, bits).Masked().Addr().AsSlice()
	network[hostBit/8] |= byte(1) << (7 - uint(hostBit%8))
	withHostBit, _ := netip.AddrFromSlice(network)
	return fmt.Sprintf("%s/%d", withHostBit, bits)
}

// TestProperty_ParseTargetHostPrefixes_NormalizesAcceptedEntries asserts the
// accepting path is correct, not merely non-erroring: a bare IP becomes its
// single-host prefix, a canonical CIDR is preserved, surrounding whitespace is
// trimmed, and order and count are preserved. The rejection property
// (TestProperty_ParseTargetHostPrefixes_RejectsHostileEntries) covers the other
// half; a normalization bug such as a bare IP widened to the wrong prefix length
// would fail here.
func TestProperty_ParseTargetHostPrefixes_NormalizesAcceptedEntries(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(t, "n")
		inputs := make([]string, n)
		want := make([]netip.Prefix, n)
		for i := range inputs {
			s, p := genAcceptedEntry(t)
			if rapid.Bool().Draw(t, "pad") {
				s = "  " + s + "\t"
			}
			inputs[i] = s
			want[i] = p
		}

		got, err := ParseTargetHostPrefixes(inputs)
		require.NoError(t, err, "accepted inputs were rejected: %v", inputs)
		require.Equal(t, want, got, "parsed prefixes did not match expected: inputs=%v", inputs)
	})
}

// genAcceptedEntry draws a config entry the parser accepts, paired with the
// prefix it must produce: a bare IP maps to its single-host prefix, a canonical
// CIDR maps to itself.
func genAcceptedEntry(t *rapid.T) (string, netip.Prefix) {
	addr := genCleanAddr(t)
	if rapid.Bool().Draw(t, "bare") {
		return addr.String(), netip.PrefixFrom(addr, addr.BitLen())
	}
	bits := rapid.IntRange(0, addr.BitLen()).Draw(t, "bits")
	p := netip.PrefixFrom(addr, bits).Masked()
	return p.String(), p
}

// genValidEntries draws a list of entries the parser accepts: bare canonical IPs
// and canonical CIDRs, none carrying a zone or IPv4-in-IPv6 notation.
func genValidEntries(t *rapid.T) []string {
	n := rapid.IntRange(0, 4).Draw(t, "n_valid")
	entries := make([]string, 0, n)
	for range n {
		addr := genCleanAddr(t)
		if rapid.Bool().Draw(t, "bare") {
			entries = append(entries, addr.String())
			continue
		}
		bits := rapid.IntRange(0, addr.BitLen()).Draw(t, "valid_bits")
		entries = append(entries, netip.PrefixFrom(addr, bits).Masked().String())
	}
	return entries
}

// genCleanAddr draws an address in the canonical form the parser accepts: an
// unmapped IPv4 or IPv6 address with no zone.
func genCleanAddr(t *rapid.T) netip.Addr {
	if rapid.Bool().Draw(t, "clean_v4") {
		return genV4(t)
	}
	return genV6(t).Unmap()
}

func genV4(t *rapid.T) netip.Addr {
	return netip.AddrFrom4([4]byte(rapid.SliceOfN(rapid.Byte(), 4, 4).Draw(t, "v4_bytes")))
}

func genV6(t *rapid.T) netip.Addr {
	return netip.AddrFrom16([16]byte(rapid.SliceOfN(rapid.Byte(), 16, 16).Draw(t, "v6_bytes")))
}
