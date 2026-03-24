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

package winpki

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net"
	"testing"

	"github.com/go-ldap/ldap/v3"
	"github.com/russellhaering/gosaml2/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	libSet "github.com/gravitational/teleport/lib/utils/set"
	libslices "github.com/gravitational/teleport/lib/utils/slices"
)

type result struct {
	entries []*ldap.Entry
	// strings that index into a referrals map
	referral []string
}

// referrals tracks fake LDAP referrals which can be
// resolved to some number of hosts with the accompanied
// LookupSRV method. This allows us to mock SRV lookups.
type referrals struct {
	refs     map[string][]string
	resolves map[string]int
}

func (r referrals) LookupSRV(ctx context.Context, _, _, name string) (string, []*net.SRV, error) {
	r.resolves[name] += 1
	if len(r.refs[name]) == 0 {
		return "", nil, errors.New("not found")
	}

	return "", libslices.Map(r.refs[name], func(s string) *net.SRV {
		return &net.SRV{
			Target: s,
		}
	}), ctx.Err()
}

func (r *referrals) clear() {
	clear(r.resolves)
}

// topology represents an LDAP server topology,
// wherein a given host, when searched, returns either
// LDAP entries or referrals.
type topology struct {
	servers     map[string]result
	searches    map[string]int
	openClients map[string]struct{}
}

func (t *topology) clear() {
	clear(t.searches)
	clear(t.openClients)
}

func (t topology) newSearcher(_ context.Context, name string) (searcher, func() error, error) {
	id := uuid.NewV4().String()
	t.openClients[id] = struct{}{}
	return searcherFunc(func(ctx context.Context, searchRequest *ldap.SearchRequest) (entries []*ldap.Entry, referrals []string, err error) {
		res, ok := t.servers[name]
		t.searches[name] += 1
		if !ok {
			return nil, nil, errors.New("search failed for this host")
		}

		return res.entries, res.referral, ctx.Err()
	}), func() error { delete(t.openClients, id); return nil }, nil
}

func TestRecursiveSearch(t *testing.T) {
	t.Parallel()

	discardLogger := slog.New(slog.DiscardHandler)

	// Define a mapping of referrals to resolved hostnames to use for testing.
	refs := referrals{
		resolves: map[string]int{},
		refs: map[string][]string{
			"a.lab.local": []string{"hosta1", "hosta2", "hosta3", "hosta4"},
			"b.lab.local": []string{"hostb1", "hostb2", "hostb3"},
			"c.lab.local": []string{"hostc1", "hostc2"},
			"d.lab.local": []string{"hostd1"},
			// Test fallback behavior where a failed SRV lookup
			// falls back to the parsed url.
			"noresolve.com": nil,
		},
	}

	// Define a topology for this test. If started from root and search
	// limits allow, the recursive search would exhaustively query each "server",
	// visiting every host, until it finds the single valid LDAP entry held by
	// "noresolve.com".
	top := topology{
		servers: map[string]result{
			"root": result{
				referral: []string{"ldaps://a.lab.local"},
			},
			"hosta4": result{
				referral: []string{"ldaps://b.lab.local"},
			},
			"hostb3": result{
				referral: []string{"ldaps://c.lab.local"},
			},
			"hostc2": result{
				referral: []string{"ldaps://d.lab.local"},
			},
			"hostd1": result{
				referral: []string{"ldaps://noresolve.com"},
			},
			"noresolve.com": result{
				entries: []*ldap.Entry{&ldap.Entry{
					DN: "somedn",
				}},
			},
		},
		searches:    map[string]int{},
		openClients: map[string]struct{}{},
	}

	t.Run("successful search", func(t *testing.T) {
		testSearch := recursiveSearch{
			maxDepth:     100,
			maxHosts:     100,
			maxReferrals: 100,
			referrals:    make(libSet.Set[string]),
			request:      &ldap.SearchRequest{},
			newSearcher:  top.newSearcher,
			resolver:     refs,
			logger:       discardLogger,
		}

		// Start the search from the "root" mock LDAPS server
		root, closer, _ := top.newSearcher(t.Context(), "root")
		closer()
		entries, err := testSearch.start(t.Context(), root)
		require.NoError(t, err)
		assert.Len(t, entries, 1)
		// Should have followed all 5 referrals
		assert.Len(t, testSearch.referrals, 5)
		// All client handles closed
		assert.Empty(t, top.openClients)
	})

	t.Run("max referrals exceeded", func(t *testing.T) {
		refs.clear()
		top.clear()

		// Should fail after trying two referrals
		testSearch := recursiveSearch{
			maxDepth:     100,
			maxHosts:     100,
			maxReferrals: 2,
			referrals:    make(libSet.Set[string]),
			request:      &ldap.SearchRequest{},
			newSearcher:  top.newSearcher,
			resolver:     refs,
			logger:       discardLogger,
		}

		// Start the search from the "root" mock LDAPS server
		root, closer, _ := top.newSearcher(t.Context(), "root")
		closer()
		entries, err := testSearch.start(t.Context(), root)
		assert.NoError(t, err)
		assert.Empty(t, entries)
		// As a quirk, the third referral ends up on the referral list (since we *did* see it),
		// but is not actually visited
		for _, key := range []string{"ldaps://a.lab.local", "ldaps://b.lab.local", "ldaps://c.lab.local"} {
			assert.Contains(t, testSearch.referrals, key)
		}
		// Validates that c.lab.local was never resolved
		assert.NotContains(t, refs.resolves, "c.lab.local")
		// All client handles closed
		assert.Empty(t, top.openClients)
	})

	t.Run("max hosts exceeded", func(t *testing.T) {
		refs.clear()
		top.clear()

		// Should fail on the first referral since only the last host
		// "hosta4" contains the next referral
		testSearch := recursiveSearch{
			maxDepth:     100,
			maxHosts:     3,
			maxReferrals: 100,
			referrals:    make(libSet.Set[string]),
			request:      &ldap.SearchRequest{},
			newSearcher:  top.newSearcher,
			resolver:     refs,
			logger:       discardLogger,
		}

		// Start the search from the "root" mock LDAPS server
		root, closer, _ := top.newSearcher(t.Context(), "root")
		closer()
		entries, err := testSearch.start(t.Context(), root)
		assert.NoError(t, err)
		assert.Empty(t, entries)
		for _, key := range []string{"ldaps://a.lab.local"} {
			assert.Contains(t, testSearch.referrals, key)
		}
		// Validates that b.lab.local was never resolved because the search failed
		// before this referral was discovered.
		assert.NotContains(t, refs.resolves, "b.lab.local")
		// All client handles closed
		assert.Empty(t, top.openClients)
	})

	t.Run("max depth exceeded", func(t *testing.T) {
		refs.clear()
		top.clear()

		// Should fail just before reaching the end
		testSearch := recursiveSearch{
			maxDepth:     4,
			maxHosts:     100,
			maxReferrals: 100,
			referrals:    make(libSet.Set[string]),
			request:      &ldap.SearchRequest{},
			newSearcher:  top.newSearcher,
			resolver:     refs,
			logger:       discardLogger,
		}

		// Start the search from the "root" mock LDAPS server
		root, closer, _ := top.newSearcher(t.Context(), "root")
		closer()
		entries, err := testSearch.start(t.Context(), root)
		assert.NoError(t, err)
		assert.Empty(t, entries)
		for _, key := range []string{"ldaps://a.lab.local", "ldaps://b.lab.local", "ldaps://c.lab.local", "ldaps://d.lab.local"} {
			assert.Contains(t, testSearch.referrals, key)
		}
		assert.NotContains(t, refs.resolves, "noresolve.com")
		// All client handles closed
		assert.Empty(t, top.openClients)
	})

	t.Run("context cancellation returns error", func(t *testing.T) {
		refs.clear()
		top.clear()

		subCtx, cancel := context.WithCancel(t.Context())
		testSearch := recursiveSearch{
			maxDepth:     100,
			maxHosts:     100,
			maxReferrals: 100,
			referrals:    make(libSet.Set[string]),
			request:      &ldap.SearchRequest{},
			newSearcher:  top.newSearcher,
			resolver:     refs,
			logger:       discardLogger,
		}

		cancel()
		// Start the search from the "root" mock LDAPS server
		root, closer, _ := top.newSearcher(t.Context(), "root")
		closer()
		entries, err := testSearch.start(subCtx, root)
		assert.Error(t, err)
		assert.Empty(t, entries)
		// All client handles closed
		assert.Empty(t, top.openClients)
	})

	t.Run("cycles skipped", func(t *testing.T) {
		cycleRefs := referrals{
			resolves: map[string]int{},
			refs: map[string][]string{
				"a.lab.local": []string{"hosta1"},
				"b.lab.local": []string{"hostb1"},
			},
		}

		cycleTopology := topology{
			searches:    map[string]int{},
			openClients: map[string]struct{}{},
			servers: map[string]result{
				"root":   result{referral: []string{"ldaps://a.lab.local"}},
				"hosta1": result{referral: []string{"ldaps://b.lab.local"}},
				// points back to a previous referral
				"hostb1": result{referral: []string{"ldaps://a.lab.local"}},
			},
		}

		// Should fail just before reaching the end
		testSearch := recursiveSearch{
			maxDepth:     math.MaxInt32,
			maxHosts:     100,
			maxReferrals: math.MaxInt32,
			referrals:    make(libSet.Set[string]),
			request:      &ldap.SearchRequest{},
			newSearcher:  cycleTopology.newSearcher,
			resolver:     cycleRefs,
			logger:       slog.Default(),
		}

		// Start the search from the "root" mock LDAPS server
		root, closer, _ := top.newSearcher(t.Context(), "root")
		closer()
		entries, err := testSearch.start(t.Context(), root)
		require.NoError(t, err)
		assert.Empty(t, entries)
		// Referral and associated host are only resolved/searched once
		assert.Equal(t, 1, cycleRefs.resolves["a.lab.local"])
		assert.Equal(t, 1, cycleTopology.searches["hosta1"])
		// All client handles closed
		assert.Empty(t, top.openClients)
	})
}
