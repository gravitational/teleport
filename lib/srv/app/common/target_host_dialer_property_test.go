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
	"context"
	"net"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestProperty_DialAttempt_MatchesReferenceModel checks the candidate classifier
// against an independent fold. dialAttempt accumulates state incrementally under
// a mutex with an early "first blocked" capture; the model computes the same
// decision as a plain pass over the sequence. For any policy and any order of
// candidates, control's per-candidate result and denial()'s verdict and audit
// metadata must equal the model. This generalizes the hand-picked cases in
// TestTargetDialerControl (allowed short-circuits, all-blocked, no candidates).
func TestProperty_DialAttempt_MatchesReferenceModel(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		policy := genPolicy(t)
		cands := genCandidates(t, policy)

		a := &dialAttempt{policy: policy, host: "target.example.com", port: "443"}
		gotErr := make([]bool, len(cands))
		for i, c := range cands {
			err := a.control(context.Background(), "tcp", net.JoinHostPort(c.String(), "443"), nil)
			gotErr[i] = err != nil
		}

		// Reference model: a straight fold over the same candidates.
		var resolved []netip.Addr
		allowedSeen, hasBlocked := false, false
		var firstBlocked netip.Addr
		for i, c := range cands {
			canon := canonicalAddr(c)
			resolved = append(resolved, canon)
			blocked := rejected(policy, canon)
			require.Equal(t, blocked, gotErr[i],
				"control error must track blocked status at %d: cand=%v policy=%+v", i, c, policy)
			switch {
			case !blocked:
				allowedSeen = true
			case !hasBlocked:
				hasBlocked = true
				firstBlocked = canon
			}
		}

		denial, ok := a.denial()
		require.Equal(t, hasBlocked && !allowedSeen, ok,
			"denial verdict mismatch: cands=%v policy=%+v", cands, policy)
		if !ok {
			return
		}
		require.Equal(t, resolved, denial.ResolvedIPs, "resolved IPs mismatch: cands=%v", cands)
		require.Equal(t, firstBlocked, denial.BlockedIP, "blocked IP must be the first blocked candidate: cands=%v", cands)
		require.Equal(t, policy.mode(), denial.Policy, "policy mode mismatch")
		require.Equal(t, policy.deniedPrefix(firstBlocked), denial.BlockedPrefix, "blocked prefix mismatch")
	})
}

// TestProperty_DialAttempt_ConcurrentControlIsConsistent feeds the candidates
// through control from concurrent goroutines, mirroring the dual-stack dialer.
// The denial verdict must be order-independent, every candidate must be recorded
// exactly once, and any reported blocked IP must be one that the policy blocks.
// Run with -race to exercise the mutex.
func TestProperty_DialAttempt_ConcurrentControlIsConsistent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		policy := genPolicy(t)
		cands := genCandidates(t, policy)

		a := &dialAttempt{policy: policy, host: "target.example.com", port: "443"}
		var wg sync.WaitGroup
		for _, c := range cands {
			wg.Go(func() {
				_ = a.control(context.Background(), "tcp", net.JoinHostPort(c.String(), "443"), nil)
			})
		}
		wg.Wait()

		wantResolved := make([]netip.Addr, len(cands))
		allowedSeen, hasBlocked := false, false
		blocked := make(map[netip.Addr]bool)
		for i, c := range cands {
			canon := canonicalAddr(c)
			wantResolved[i] = canon
			if rejected(policy, canon) {
				hasBlocked = true
				blocked[canon] = true
			} else {
				allowedSeen = true
			}
		}

		// All goroutines have joined, so reading a.resolved is race-free.
		require.ElementsMatch(t, wantResolved, a.resolved,
			"every candidate must be recorded exactly once: cands=%v", cands)

		denial, ok := a.denial()
		require.Equal(t, hasBlocked && !allowedSeen, ok,
			"denial verdict must not depend on control ordering: cands=%v policy=%+v", cands, policy)
		if ok {
			require.True(t, blocked[denial.BlockedIP],
				"reported blocked IP %v is not blocked by the policy: cands=%v", denial.BlockedIP, cands)
		}
	})
}

// TestProperty_DialAttempt_EvaluatesCanonicalForm asserts the classifier both
// records and evaluates the canonical form of every candidate, whatever
// presentation the OS supplies. For each hostile form fed to control, the
// recorded address must be the canonical base and the control verdict must match
// the policy applied to that base. This catches a classifier that records the
// canonical address but tests the policy against the raw one, which would let an
// IPv4-mapped or zoned form of a denied host slip through.
func TestProperty_DialAttempt_EvaluatesCanonicalForm(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		policy := genPolicy(t)
		n := rapid.IntRange(1, 5).Draw(t, "n_candidates")

		bases := make([]netip.Addr, n)
		gotErr := make([]bool, n)
		a := &dialAttempt{policy: policy, host: "target.example.com", port: "443"}
		for i := range bases {
			base := canonicalAddr(genAddr(t))
			bases[i] = base
			form := oneHostileForm(t, base)
			err := a.control(context.Background(), "tcp", net.JoinHostPort(form.String(), "443"), nil)
			gotErr[i] = err != nil
		}

		require.Len(t, a.resolved, n, "every candidate must be recorded exactly once")
		for i, base := range bases {
			require.Equal(t, base, a.resolved[i],
				"control must record the canonical form: base=%v recorded=%v", base, a.resolved[i])
			require.Equal(t, rejected(policy, base), gotErr[i],
				"control must evaluate the policy on the canonical form: base=%v policy=%+v", base, policy)
		}
	})
}

// genPolicy draws an enabled single-list policy (allow or deny) of canonical
// prefixes, as dialAttempt only runs when the policy is enabled.
func genPolicy(t *rapid.T) TargetHostPolicy {
	n := rapid.IntRange(1, 4).Draw(t, "n_policy_prefixes")
	prefixes := make([]netip.Prefix, n)
	for i := range prefixes {
		prefixes[i] = genCanonicalPrefix(t)
	}
	if rapid.Bool().Draw(t, "is_allow") {
		return TargetHostPolicy{AllowedPrefixes: prefixes}
	}
	return TargetHostPolicy{DeniedPrefixes: prefixes}
}

// genCandidates draws a candidate sequence biased so that blocked, permitted, and
// unspecified addresses all occur: some are built inside a policy prefix, some are
// arbitrary, and some are the unspecified address (which the classifier must
// always reject). Each may then be presented in a hostile (mapped or zoned) form.
func genCandidates(t *rapid.T, policy TargetHostPolicy) []netip.Addr {
	prefixes := policy.AllowedPrefixes
	if len(prefixes) == 0 {
		prefixes = policy.DeniedPrefixes
	}
	n := rapid.IntRange(0, 6).Draw(t, "n_candidates")
	cands := make([]netip.Addr, n)
	for i := range cands {
		var c netip.Addr
		switch rapid.IntRange(0, 2).Draw(t, "candidate_kind") {
		case 0:
			p := prefixes[rapid.IntRange(0, len(prefixes)-1).Draw(t, "which_prefix")]
			c = addrInPrefix(t, p)
		case 1:
			c = canonicalAddr(genAddr(t))
		default:
			c = rapid.SampledFrom([]netip.Addr{netip.IPv4Unspecified(), netip.IPv6Unspecified()}).Draw(t, "unspecified")
		}
		// Sometimes present the candidate the way an OS might, mapped or zoned.
		// The reference model canonicalizes every candidate, so this exercises the
		// classifier's own canonicalization without changing the expected verdict.
		if rapid.Bool().Draw(t, "hostile") {
			c = oneHostileForm(t, canonicalAddr(c))
		}
		cands[i] = c
	}
	return cands
}

// rejected mirrors dialAttempt.control's per-candidate decision: an unspecified
// address is never dialable (on Linux it reaches loopback), and otherwise the
// policy's membership check decides.
func rejected(policy TargetHostPolicy, addr netip.Addr) bool {
	return addr.IsUnspecified() || policy.blocked(addr)
}

// oneHostileForm draws a single presentation of base: the address itself, a
// zone-tagged form, or (for IPv4) an IPv4-mapped form.
func oneHostileForm(t *rapid.T, base netip.Addr) netip.Addr {
	forms := hostileForms(t, base)
	return forms[rapid.IntRange(0, len(forms)-1).Draw(t, "which_form")]
}
