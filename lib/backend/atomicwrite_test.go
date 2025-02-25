/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package backend

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtomicWriteValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		condacts []ConditionalAction
		ok       bool
		desc     string
		estr     string
	}

	tts := []testCase{
		{
			condacts: []ConditionalAction{
				{
					Key:       NewKey("foo", "bar"),
					Condition: Exists(),
					Action: Put(Item{
						Value: []byte("true"),
					}),
				},
				{
					Key:       NewKey("bin", "baz"),
					Condition: Revision("r1"),
					Action:    Delete(),
				},
				{
					Key:       NewKey("apples", "oranges"),
					Condition: NotExists(),
					Action:    Nop(),
				},
				{
					Key:       NewKey("up", "down"),
					Condition: Whatever(),
					Action: Put(Item{
						Value: []byte("v"),
					}),
				},
			},
			ok:   true,
			desc: "basic case",
		},
		{
			condacts: []ConditionalAction{
				{
					Key:       NewKey("foo", "bar"),
					Condition: Revision(""), // empty revisions are allowed
					Action:    Delete(),
				},
			},
			ok:   true,
			desc: "empty revision",
		},
		{
			condacts: []ConditionalAction{
				{
					Key:       NewKey("singleton"),
					Condition: Whatever(),
					Action:    Delete(),
				},
			},
			ok:   true,
			desc: "singleton",
		},
		{
			condacts: nil,
			ok:       false,
			desc:     "empty",
			estr:     "empty conditional action list",
		},
		{
			condacts: []ConditionalAction{
				{
					Key:       NewKey("foo", "bar"),
					Condition: Exists(),
					Action: Put(Item{
						Value: []byte("true"),
					}),
				},
				{
					Key:       NewKey("foo", "baz"),
					Condition: Revision("r1"),
					Action:    Delete(),
				},
				{
					Key:       NewKey("apples", "oranges"),
					Condition: NotExists(),
					Action:    Nop(),
				},
				{
					Key:       NewKey("foo", "bar"),
					Condition: Revision("r2"),
					Action:    Nop(),
				},
			},
			ok:   false,
			desc: "duplicate keys",
			estr: "multiple conditional actions target key",
		},
		{
			condacts: []ConditionalAction{
				{
					Key:       NewKey("foo", "bar"),
					Condition: Exists(),
					Action:    Put(Item{}),
				},
			},
			ok:   false,
			desc: "empty put",
			estr: "missing required put parameter Item.Value",
		},
		{
			condacts: []ConditionalAction{
				{
					Key:       NewKey("foo", "bar"),
					Condition: Condition{},
					Action:    Delete(),
				},
			},
			ok:   false,
			desc: "zero condition",
			estr: "missing required parameter 'Condition'",
		},
		{
			condacts: []ConditionalAction{
				{
					Key:       NewKey("foo", "bar"),
					Condition: Exists(),
					Action:    Action{},
				},
			},
			ok:   false,
			desc: "zero action",
			estr: "missing required parameter 'Action'",
		},
	}

	var big []ConditionalAction
	for i := 0; i < MaxAtomicWriteSize+1; i++ {
		big = append(big, ConditionalAction{
			Key:       NewKey("key-" + strconv.Itoa(i)),
			Condition: Whatever(),
			Action:    Delete(),
		})
	}

	tts = append(tts, testCase{
		condacts: big,
		ok:       false,
		desc:     "too big",
	})

	tts = append(tts, testCase{
		condacts: big[:MaxAtomicWriteSize],
		ok:       true,
		desc:     "max",
	})

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			err := ValidateAtomicWrite(tt.condacts)
			if tt.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.estr)
			}
		})
	}
}
