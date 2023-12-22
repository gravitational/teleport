/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
	"testing"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

type wrapRecipientsMap struct {
	RecipientsMap RawRecipientsMap `toml:"role_to_recipients"`
}

func TestRawRecipientsMap(t *testing.T) {
	testCases := []struct {
		desc             string
		in               string
		expectRecipients RawRecipientsMap
	}{
		{
			desc: "test role_to_recipients multiple format",
			in: `
            [role_to_recipients]
            "dev" = ["dev-channel", "admin-channel"]
            "*" = "admin-channel"
            `,
			expectRecipients: RawRecipientsMap{
				"dev":          []string{"dev-channel", "admin-channel"},
				types.Wildcard: []string{"admin-channel"},
			},
		},
		{
			desc: "test role_to_recipients role to list of recipients",
			in: `
            [role_to_recipients]
            "dev" = ["dev-channel", "admin-channel"]
            "prod" = ["sre-channel", "oncall-channel"]
            `,
			expectRecipients: RawRecipientsMap{
				"dev":  []string{"dev-channel", "admin-channel"},
				"prod": []string{"sre-channel", "oncall-channel"},
			},
		},
		{
			desc: "test role_to_recipients role to string recipient",
			in: `
            [role_to_recipients]
            "single" = "admin-channel"
            `,
			expectRecipients: RawRecipientsMap{
				"single": []string{"admin-channel"},
			},
		},
		{
			desc: "test role_to_recipients multiple format",
			in: `
            [role_to_recipients]
            "dev" = ["dev-channel", "admin-channel"]
            "*" = "admin-channel"
            `,
			expectRecipients: RawRecipientsMap{
				"dev":          []string{"dev-channel", "admin-channel"},
				types.Wildcard: []string{"admin-channel"},
			},
		},
		{
			desc: "test role_to_recipients no mapping",
			in: `
            [role_to_recipients]
            `,
			expectRecipients: RawRecipientsMap{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			w := wrapRecipientsMap{}
			err := toml.Unmarshal([]byte(tc.in), &w)
			require.NoError(t, err)

			require.Equal(t, tc.expectRecipients, w.RecipientsMap)
		})
	}
}

func TestRawRecipientsMapGetRecipients(t *testing.T) {
	testCases := []struct {
		desc               string
		m                  RawRecipientsMap
		roles              []string
		suggestedReviewers []string
		output             []string
	}{
		{
			desc: "test match exact role",
			m: RawRecipientsMap{
				"dev": []string{"chanDev"},
				"*":   []string{"chanA", "chanB"},
			},
			roles:              []string{"dev"},
			suggestedReviewers: []string{},
			output:             []string{"chanDev"},
		},
		{
			desc: "test only default recipient",
			m: RawRecipientsMap{
				"*": []string{"chanA", "chanB"},
			},
			roles:              []string{"dev"},
			suggestedReviewers: []string{},
			output:             []string{"chanA", "chanB"},
		},
		{
			desc: "test deduplicate recipients",
			m: RawRecipientsMap{
				"dev": []string{"chanA", "chanB"},
				"*":   []string{"chanC"},
			},
			roles:              []string{"dev"},
			suggestedReviewers: []string{"chanA", "chanB"},
			output:             []string{"chanA", "chanB"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			recipients := tc.m.GetRawRecipientsFor(tc.roles, tc.suggestedReviewers)
			require.ElementsMatch(t, recipients, tc.output)
		})
	}
}

func TestNewRecipientSet(t *testing.T) {
	actual := NewRecipientSet()
	expected := RecipientSet{recipients: make(map[string]Recipient)}
	require.Equal(t, expected, actual)
}

func TestRecipientSet_Add(t *testing.T) {
	// Setup
	set := NewRecipientSet()
	a := Recipient{
		Name: "Recipient A",
		ID:   "A",
		Kind: "Test",
	}
	b := Recipient{
		Name: "Recipient B",
		ID:   "B",
		Kind: "Test",
	}
	a2 := Recipient{
		Name: "Recipient A2",
		ID:   "A",
		Kind: "Test",
		Data: nil,
	}

	// Testing with a single element
	set.Add(a)
	require.Equal(t, map[string]Recipient{"A": a}, set.recipients)

	// Testing with a second element
	set.Add(b)
	require.Equal(t, map[string]Recipient{"A": a, "B": b}, set.recipients)

	// Testing with an element with the same ID
	set.Add(a2)
	require.Equal(t, map[string]Recipient{"A": a2, "B": b}, set.recipients)
}

func TestRecipientSet_Contains(t *testing.T) {
	// Setup
	a := Recipient{
		Name: "Recipient A",
		ID:   "A",
		Kind: "Test",
	}
	b := Recipient{
		Name: "Recipient B",
		ID:   "B",
		Kind: "Test",
	}
	set := RecipientSet{recipients: map[string]Recipient{"A": a, "B": b}}

	// Testing contains on a couple elements
	require.True(t, set.Contains(a.ID))
	require.True(t, set.Contains(b.ID))

	// Testing contains on an absent element
	require.False(t, set.Contains("non-existent"))
}

func TestRecipientSet_ToSlice(t *testing.T) {
	// Setup
	emptySet := NewRecipientSet()
	a := Recipient{
		Name: "Recipient A",
		ID:   "A",
		Kind: "Test",
	}
	b := Recipient{
		Name: "Recipient B",
		ID:   "B",
		Kind: "Test",
	}
	set := RecipientSet{recipients: map[string]Recipient{"A": a, "B": b}}

	// Testing with an empty set
	require.Equal(t, []Recipient{}, emptySet.ToSlice())
	// Testing with a non-empty set
	require.ElementsMatch(t, []Recipient{a, b}, set.ToSlice())
}
