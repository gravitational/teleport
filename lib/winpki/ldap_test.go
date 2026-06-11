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
	servers     map[string]searchResult
	searches    map[string]int
	openClients map[string]struct{}
}

func (t *topology) clear() {
	clear(t.searches)
	clear(t.openClients)
}

type funcSearcher struct {
	searchFunc func(ctx context.Context, searchRequest *ldap.SearchRequest) (res searchResult, err error)
	onClose    func() error
}

func (f funcSearcher) search(ctx context.Context, searchRequest *ldap.SearchRequest) (res searchResult, err error) {
	return f.searchFunc(ctx, searchRequest)
}

func (f funcSearcher) Close() error {
	return f.onClose()
}

func (t topology) newSearcher(_ context.Context, name string) (searcher, error) {
	id := uuid.NewV4().String()
	t.openClients[id] = struct{}{}
	return funcSearcher{searchFunc: func(ctx context.Context, searchRequest *ldap.SearchRequest) (res searchResult, err error) {
		res, ok := t.servers[name]
		t.searches[name] += 1
		if !ok {
			return nil, errors.New("search failed for this host")
		}

		return res, ctx.Err()
	}, onClose: func() error { delete(t.openClients, id); return nil }}, nil
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
		servers: map[string]searchResult{
			"root": searchResultReferral{
				referrals: []string{"ldaps://a.lab.local"},
			},
			"hosta4": searchResultReferral{
				referrals: []string{"ldaps://b.lab.local"},
			},
			"hostb3": searchResultReferral{
				referrals: []string{"ldaps://c.lab.local"},
			},
			"hostc2": searchResultReferral{
				referrals: []string{"ldaps://d.lab.local"},
			},
			"hostd1": searchResultReferral{
				referrals: []string{"ldaps://noresolve.com"},
			},
			"noresolve.com": searchResultEntry{
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
			newSearcher:  top.newSearcher,
			resolver:     refs,
			logger:       discardLogger,
		}

		// Start the search from the "root" mock LDAPS server
		root, _ := top.newSearcher(t.Context(), "root")
		root.Close()
		entries, err := testSearch.start(t.Context(), root, &ldap.SearchRequest{})
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
			newSearcher:  top.newSearcher,
			resolver:     refs,
			logger:       discardLogger,
		}

		// Start the search from the "root" mock LDAPS server
		root, _ := top.newSearcher(t.Context(), "root")
		root.Close()
		entries, err := testSearch.start(t.Context(), root, &ldap.SearchRequest{})
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
			newSearcher:  top.newSearcher,
			resolver:     refs,
			logger:       discardLogger,
		}

		// Start the search from the "root" mock LDAPS server
		root, _ := top.newSearcher(t.Context(), "root")
		root.Close()
		entries, err := testSearch.start(t.Context(), root, &ldap.SearchRequest{})
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
			newSearcher:  top.newSearcher,
			resolver:     refs,
			logger:       discardLogger,
		}

		// Start the search from the "root" mock LDAPS server
		root, _ := top.newSearcher(t.Context(), "root")
		root.Close()
		entries, err := testSearch.start(t.Context(), root, &ldap.SearchRequest{})
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
			newSearcher:  top.newSearcher,
			resolver:     refs,
			logger:       discardLogger,
		}

		cancel()
		// Start the search from the "root" mock LDAPS server
		root, _ := top.newSearcher(t.Context(), "root")
		root.Close()
		entries, err := testSearch.start(subCtx, root, &ldap.SearchRequest{})
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
			servers: map[string]searchResult{
				"root":   searchResultReferral{referrals: []string{"ldaps://a.lab.local"}},
				"hosta1": searchResultReferral{referrals: []string{"ldaps://b.lab.local"}},
				// points back to a previous referral
				"hostb1": searchResultReferral{referrals: []string{"ldaps://a.lab.local"}},
			},
		}

		// Should fail just before reaching the end
		testSearch := recursiveSearch{
			maxDepth:     math.MaxInt32,
			maxHosts:     100,
			maxReferrals: math.MaxInt32,
			referrals:    make(libSet.Set[string]),
			newSearcher:  cycleTopology.newSearcher,
			resolver:     cycleRefs,
			logger:       slog.Default(),
		}

		// Start the search from the "root" mock LDAPS server
		root, _ := top.newSearcher(t.Context(), "root")
		root.Close()
		entries, err := testSearch.start(t.Context(), root, &ldap.SearchRequest{})
		require.NoError(t, err)
		assert.Empty(t, entries)
		// Referral and associated host are only resolved/searched once
		assert.Equal(t, 1, cycleRefs.resolves["a.lab.local"])
		assert.Equal(t, 1, cycleTopology.searches["hosta1"])
		// All client handles closed
		assert.Empty(t, top.openClients)
	})
}

func TestReferralParsing(t *testing.T) {
	t.Run("full url", func(t *testing.T) {
		ref, err := parseLDAPReferral("ldaps://somehost:123/DC=example,DC=com?cn,mail?sub?filter")
		require.NoError(t, err)
		assert.Equal(t, "ldaps://", ref.scheme)
		assert.Equal(t, "somehost:123", ref.host)
		assert.Equal(t, "DC=example,DC=com", ref.baseDN)
		assert.Equal(t, "cn,mail", ref.attributes)
		assert.Equal(t, "filter", ref.filter)
		assert.Equal(t, "sub", ref.scope)
	})

	t.Run("attributes only", func(t *testing.T) {
		ref, err := parseLDAPReferral("ldaps://somehost:123/dn?cn,mail")
		require.NoError(t, err)
		assert.Equal(t, "ldaps://", ref.scheme)
		assert.Equal(t, "somehost:123", ref.host)
		assert.Equal(t, "dn", ref.baseDN)
		assert.Equal(t, "cn,mail", ref.attributes)
		assert.Empty(t, ref.filter)
		assert.Empty(t, ref.scope)
	})

	t.Run("scope only", func(t *testing.T) {
		ref, err := parseLDAPReferral("ldaps://somehost:123/DC=example,DC=com??one")
		require.NoError(t, err)
		assert.Equal(t, "ldaps://", ref.scheme)
		assert.Equal(t, "somehost:123", ref.host)
		assert.Equal(t, "DC=example,DC=com", ref.baseDN)
		assert.Empty(t, ref.attributes)
		assert.Empty(t, ref.filter)
		assert.Equal(t, "one", ref.scope)
	})

	t.Run("filter only", func(t *testing.T) {
		ref, err := parseLDAPReferral("ldaps://somehost:123/DC=example,DC=com???filter")
		require.NoError(t, err)
		assert.Equal(t, "ldaps://", ref.scheme)
		assert.Equal(t, "somehost:123", ref.host)
		assert.Equal(t, "DC=example,DC=com", ref.baseDN)
		assert.Empty(t, ref.attributes)
		assert.Equal(t, "filter", ref.filter)
		assert.Empty(t, ref.scope)
	})

	t.Run("attributes and filter only", func(t *testing.T) {
		ref, err := parseLDAPReferral("ldaps://somehost:123/DC=example,DC=com?cn,mail??filter")
		require.NoError(t, err)
		assert.Equal(t, "ldaps://", ref.scheme)
		assert.Equal(t, "somehost:123", ref.host)
		assert.Equal(t, "DC=example,DC=com", ref.baseDN)
		assert.Equal(t, "cn,mail", ref.attributes)
		assert.Equal(t, "filter", ref.filter)
		assert.Empty(t, ref.scope)
	})

	t.Run("extensions only - with dn", func(t *testing.T) {
		ref, err := parseLDAPReferral("ldaps://somehost:123/DC=example,DC=com????extensions")
		require.NoError(t, err)
		assert.Equal(t, "ldaps://", ref.scheme)
		assert.Equal(t, "somehost:123", ref.host)
		assert.Equal(t, "DC=example,DC=com", ref.baseDN)
		assert.Empty(t, ref.attributes)
		assert.Empty(t, ref.filter)
		assert.Empty(t, ref.scope)
	})

	t.Run("extensions only", func(t *testing.T) {
		ref, err := parseLDAPReferral("ldaps://somehost:123/dn??????extensions")
		require.NoError(t, err)
		assert.Equal(t, "ldaps://", ref.scheme)
		assert.Equal(t, "somehost:123", ref.host)
		assert.Equal(t, "dn", ref.baseDN)
		assert.Empty(t, ref.attributes)
		assert.Empty(t, ref.filter)
		assert.Empty(t, ref.scope)
		// Excess '?'s end up in extensions
		assert.Equal(t, "??extensions", ref.extensions)
	})

	t.Run("dn only", func(t *testing.T) {
		ref, err := parseLDAPReferral("ldap:///dn")
		require.NoError(t, err)
		assert.Equal(t, "ldap://", ref.scheme)
		assert.Equal(t, "dn", ref.baseDN)
		assert.Empty(t, ref.host)
		assert.Empty(t, ref.attributes)
		assert.Empty(t, ref.filter)
		assert.Empty(t, ref.scope)
		assert.Empty(t, ref.extensions)
	})

	t.Run("sparse but valid", func(t *testing.T) {
		ref, err := parseLDAPReferral("ldap:///dn????")
		require.NoError(t, err)
		assert.Equal(t, "ldap://", ref.scheme)
		assert.Equal(t, "dn", ref.baseDN)
		assert.Empty(t, ref.host)
		assert.Empty(t, ref.attributes)
		assert.Empty(t, ref.filter)
		assert.Empty(t, ref.scope)
		assert.Empty(t, ref.extensions)
	})

	t.Run("scheme only", func(t *testing.T) {
		_, err := parseLDAPReferral("ldap://")
		// We expect an error on empty referral URLs
		require.Error(t, err)
	})

	t.Run("percent encoded params", func(t *testing.T) {
		ref, err := parseLDAPReferral("ldap://host.example.com:1234/dc=example%3F,dc=com?cn,o%3Dname?sub?(cn=John%20Smith)?1.2.3.4=foo%2Cbar")
		require.NoError(t, err)
		assert.Equal(t, "ldap://", ref.scheme)
		assert.Equal(t, "dc=example?,dc=com", ref.baseDN)
		assert.Equal(t, "host.example.com:1234", ref.host)
		assert.Equal(t, "cn,o=name", ref.attributes)
		assert.Equal(t, "(cn=John Smith)", ref.filter)
		assert.Equal(t, "sub", ref.scope)
		assert.Equal(t, "1.2.3.4=foo,bar", ref.extensions)
	})

	t.Run("malformed urls", func(t *testing.T) {
		for _, url := range []string{
			"ldap://host name.example.com/dc=example,dc=com", // space in host
			"ldap://host@example.com/dc=example,dc=com",      // @ in host
			"ldap://host#example.com/dc=example,dc=com",      // fragment in host
			"ldap://[::1/dc=example,dc=com",                  // unclosed IP-literal bracket
			"ldap://host?",                                   // ? in host
		} {
			_, err := parseLDAPReferral(url)
			require.Error(t, err)
		}

	})
}
