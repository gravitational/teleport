package backend

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtomicWriteValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		condacts []ConditionalAction
		ok       bool
		desc     string
	}

	tts := []testCase{
		{
			condacts: []ConditionalAction{
				{
					Key:       []byte("/foo/bar"),
					Condition: Exists(),
					Action: Put(Item{
						Value: []byte("true"),
					}),
				},
				{
					Key:       []byte("/bin/baz"),
					Condition: Revision("r1"),
					Action:    Delete(),
				},
				{
					Key:       []byte("/apples/oranges"),
					Condition: NotExists(),
					Action:    Nop(),
				},
				{
					Key:       []byte("/up/down"),
					Condition: Whatever(),
					Action:    Nop(),
				},
			},
			ok:   true,
			desc: "basic case",
		},
		{
			condacts: []ConditionalAction{
				{
					Key:       []byte("/foo/bar"),
					Condition: Revision(""), // empty revisions are allowed
					Action:    Nop(),
				},
			},
			ok:   true,
			desc: "empty revision",
		},
		{
			condacts: []ConditionalAction{
				{
					Key:       []byte("/singleton"),
					Condition: Whatever(),
					Action:    Nop(),
				},
			},
			ok:   true,
			desc: "singleton",
		},
		{
			condacts: nil,
			ok:       true,
			desc:     "empty",
		},
		{
			condacts: []ConditionalAction{
				{
					Key:       []byte("/foo/bar"),
					Condition: Exists(),
					Action: Put(Item{
						Value: []byte("true"),
					}),
				},
				{
					Key:       []byte("/foo/baz"),
					Condition: Revision("r1"),
					Action:    Delete(),
				},
				{
					Key:       []byte("/apples/oranges"),
					Condition: NotExists(),
					Action:    Nop(),
				},
				{
					Key:       []byte("/foo/bar"),
					Condition: Whatever(),
					Action:    Nop(),
				},
			},
			ok:   false,
			desc: "duplicate keys",
		},
		{
			condacts: []ConditionalAction{
				{
					Key:       []byte("/foo/bar"),
					Condition: Exists(),
					Action:    Put(Item{}),
				},
			},
			ok:   false,
			desc: "empty put",
		},
		{
			condacts: []ConditionalAction{
				{
					Key:       []byte("/foo/bar"),
					Condition: Condition{},
					Action:    Nop(),
				},
			},
			ok:   false,
			desc: "zero condition",
		},
		{
			condacts: []ConditionalAction{
				{
					Key:       []byte("/foo/bar"),
					Condition: Whatever(),
					Action:    Action{},
				},
			},
			ok:   false,
			desc: "zero action",
		},
	}

	var big []ConditionalAction
	for i := 0; i < MaxAtomicWriteSize+1; i++ {
		big = append(big, ConditionalAction{
			Key:       []byte(fmt.Sprintf("/key-%d", i)),
			Condition: Whatever(),
			Action:    Nop(),
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
		err := ValidateAtomicWrite(tt.condacts)
		if tt.ok {
			require.NoError(t, err, "desc=%q", tt.desc)
		} else {
			require.Error(t, err, "desc=%q", tt.desc)
		}
	}
}
