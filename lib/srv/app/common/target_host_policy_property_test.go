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

package common

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestProperty_CanonicalAddr_RecoversBaseAcrossForms asserts canonicalAddr maps
// every presentation of an address back to one canonical identity. A resolver or
// the OS may hand the dialer an IPv4-mapped or zone-tagged form of the same host,
// and netip.Prefix.Contains rejects both (it returns false for a zoned address
// and for an IPv4-mapped address against an IPv4 prefix), so an operator's rule
// could be evaded unless canonicalAddr neutralizes them first. Dropping either
// the zone strip or the unmap fails this property. The allow/deny verdict on the
// recovered address is exercised where the dialer applies it, in the dialAttempt
// properties.
func TestProperty_CanonicalAddr_RecoversBaseAcrossForms(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := canonicalAddr(genAddr(t))
		for _, form := range hostileForms(t, a) {
			require.Equal(t, a, canonicalAddr(form),
				"canonicalAddr did not recover the base address: a=%v form=%v", a, form)
		}
	})
}

// hostileForms returns presentations of a that a resolver or the OS could hand
// the dialer instead of the plain canonical address: for IPv4, the IPv4-mapped
// IPv6 form and that form zone-tagged; for IPv6, the zone-tagged form. WithZone
// is a no-op on IPv4, so a v4 address gets its zone coverage through the mapped
// form rather than a redundant copy of itself.
func hostileForms(t *rapid.T, a netip.Addr) []netip.Addr {
	zone := rapid.SampledFrom([]string{"eth0", "en0", "1"}).Draw(t, "zone")
	if a.Is4() {
		mapped := netip.AddrFrom16(a.As16())
		return []netip.Addr{a, mapped, mapped.WithZone(zone)}
	}
	return []netip.Addr{a, a.WithZone(zone)}
}

// genAddr draws an arbitrary IPv4 or IPv6 address from raw bytes.
func genAddr(t *rapid.T) netip.Addr {
	if rapid.Bool().Draw(t, "is_v4") {
		return netip.AddrFrom4([4]byte(rapid.SliceOfN(rapid.Byte(), 4, 4).Draw(t, "v4_bytes")))
	}
	return netip.AddrFrom16([16]byte(rapid.SliceOfN(rapid.Byte(), 16, 16).Draw(t, "v6_bytes")))
}

// genCanonicalPrefix draws a prefix in the same canonical form the config
// parser emits: masked, zone-free, and not IPv4-in-IPv6.
func genCanonicalPrefix(t *rapid.T) netip.Prefix {
	base := canonicalAddr(genAddr(t))
	bits := rapid.IntRange(0, base.BitLen()).Draw(t, "prefix_bits")
	return netip.PrefixFrom(base, bits).Masked()
}

// TestProperty_TargetHostPolicy_MembershipDecidesVerdict pins the allow/deny
// polarity against constructed ground truth: an address built to sit inside a
// prefix must be blocked by a deny list and permitted by an allow list, and one
// built to sit outside must be the reverse. An inverted branch in blocked, which
// a presentation-invariance check like canonicalAddr recovery cannot detect,
// fails here.
func TestProperty_TargetHostPolicy_MembershipDecidesVerdict(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		p := genCanonicalPrefix(t)
		deny := TargetHostPolicy{DeniedPrefixes: []netip.Prefix{p}}
		allow := TargetHostPolicy{AllowedPrefixes: []netip.Prefix{p}}

		in := addrInPrefix(t, p)
		require.True(t, p.Contains(in), "generator produced an addr outside p: p=%v in=%v", p, in)
		require.True(t, deny.blocked(in), "deny must block a member: p=%v in=%v", p, in)
		require.False(t, allow.blocked(in), "allow must permit a member: p=%v in=%v", p, in)
		require.Equal(t, p, deny.deniedPrefix(in), "deny must report the matched prefix: p=%v in=%v", p, in)

		out, ok := addrOutsidePrefix(t, p)
		if !ok {
			return // p is a /0 and contains every address of its family.
		}
		require.False(t, p.Contains(out), "generator produced an addr inside p: p=%v out=%v", p, out)
		require.False(t, deny.blocked(out), "deny must permit a non-member: p=%v out=%v", p, out)
		require.True(t, allow.blocked(out), "allow must block a non-member: p=%v out=%v", p, out)
		require.False(t, deny.deniedPrefix(out).IsValid(), "deny must not report a prefix for a non-member: p=%v out=%v", p, out)
	})
}

// TestProperty_TargetHostPolicy_AnyMatchingPrefixDecides extends the polarity
// check to multi-prefix lists: an address inside any one prefix of the list is
// treated as a member, guarding the prefixContaining fold against a bug that
// only consulted the first entry.
func TestProperty_TargetHostPolicy_AnyMatchingPrefixDecides(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(t, "n_prefixes")
		prefixes := make([]netip.Prefix, n)
		for i := range prefixes {
			prefixes[i] = genCanonicalPrefix(t)
		}
		idx := rapid.IntRange(0, n-1).Draw(t, "member_of")
		in := addrInPrefix(t, prefixes[idx])

		deny := TargetHostPolicy{DeniedPrefixes: prefixes}
		allow := TargetHostPolicy{AllowedPrefixes: prefixes}
		require.True(t, deny.blocked(in), "deny must block an addr inside prefixes[%d]: prefixes=%v in=%v", idx, prefixes, in)
		require.False(t, allow.blocked(in), "allow must permit an addr inside prefixes[%d]: prefixes=%v in=%v", idx, prefixes, in)
		require.True(t, deny.deniedPrefix(in).IsValid(), "deny must report a matched prefix: prefixes=%v in=%v", prefixes, in)
	})
}

// addrInPrefix builds an address guaranteed to lie inside p by keeping p's
// network bits and filling the host bits from random data.
func addrInPrefix(t *rapid.T, p netip.Prefix) netip.Addr {
	raw := p.Masked().Addr().AsSlice()
	host := rapid.SliceOfN(rapid.Byte(), len(raw), len(raw)).Draw(t, "host_bits")
	for i := p.Bits(); i < p.Addr().BitLen(); i++ {
		mask := byte(1) << (7 - uint(i%8))
		if host[i/8]&mask != 0 {
			raw[i/8] |= mask
		}
	}
	addr, _ := netip.AddrFromSlice(raw)
	return addr
}

// addrOutsidePrefix builds an address guaranteed to lie outside p by flipping
// one of its network bits. It reports false when p is a /0, which contains
// every address of its family and so has no outside.
func addrOutsidePrefix(t *rapid.T, p netip.Prefix) (netip.Addr, bool) {
	if p.Bits() == 0 {
		return netip.Addr{}, false
	}
	raw := p.Masked().Addr().AsSlice()
	pos := rapid.IntRange(0, p.Bits()-1).Draw(t, "flip_pos")
	raw[pos/8] ^= byte(1) << (7 - uint(pos%8))
	addr, _ := netip.AddrFromSlice(raw)
	return addr, true
}

// TestProperty_TargetHostPolicy_OppositeFamilyIsNonMember covers the negative
// multi-prefix case the single- and any-match properties miss: an address of the
// opposite family to every configured prefix is a member of none, so a deny list
// must permit it and an allow list must block it. The non-membership holds by
// construction (family mismatch), needing no netip.Contains oracle, and this
// catches a fold that treats a multi-prefix list as matching everything or that
// mishandles address family.
func TestProperty_TargetHostPolicy_OppositeFamilyIsNonMember(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		addr := canonicalAddr(genAddr(t))
		n := rapid.IntRange(1, 4).Draw(t, "n_prefixes")
		prefixes := make([]netip.Prefix, n)
		for i := range prefixes {
			prefixes[i] = genCanonicalPrefixOfFamily(t, !addr.Is4())
		}

		deny := TargetHostPolicy{DeniedPrefixes: prefixes}
		allow := TargetHostPolicy{AllowedPrefixes: prefixes}
		require.False(t, deny.blocked(addr), "deny must permit an opposite-family non-member: prefixes=%v addr=%v", prefixes, addr)
		require.True(t, allow.blocked(addr), "allow must block an opposite-family non-member: prefixes=%v addr=%v", prefixes, addr)
		require.False(t, deny.deniedPrefix(addr).IsValid(), "deny must not report a prefix for a non-member: prefixes=%v addr=%v", prefixes, addr)
	})
}

// genCanonicalPrefixOfFamily draws a canonical prefix of the requested family.
func genCanonicalPrefixOfFamily(t *rapid.T, v4 bool) netip.Prefix {
	var base netip.Addr
	if v4 {
		base = netip.AddrFrom4([4]byte(rapid.SliceOfN(rapid.Byte(), 4, 4).Draw(t, "fam_v4_bytes")))
	} else {
		b := [16]byte(rapid.SliceOfN(rapid.Byte(), 16, 16).Draw(t, "fam_v6_bytes"))
		// Force a high bit so the address is a genuine IPv6, never IPv4-in-IPv6,
		// which would collapse to IPv4 under canonicalAddr and flip its family.
		b[0] |= 0x20
		base = netip.AddrFrom16(b)
	}
	bits := rapid.IntRange(0, base.BitLen()).Draw(t, "fam_prefix_bits")
	return netip.PrefixFrom(base, bits).Masked()
}
