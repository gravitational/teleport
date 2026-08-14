package tns

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseNodes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Node
		wantErr  string
	}{
		{
			name:  "Single simple node",
			input: "(FOO=BAR)",
			expected: []Node{
				{Key: "FOO", Value: "BAR"},
			},
		},
		{
			name:  "Key with empty value",
			input: "(FOO= )",
			expected: []Node{
				{Key: "FOO", Value: ""},
			},
		},
		{
			name:  "Multiple top-level nodes",
			input: "(FIRST=1)(SECOND=2)",
			expected: []Node{
				{Key: "FIRST", Value: "1"},
				{Key: "SECOND", Value: "2"},
			},
		},
		{
			name:    "Missing equals sign",
			input:   "(FOO BAR)",
			wantErr: "key contains space",
		},
		{
			name:    "Empty key not allowed",
			input:   "(=VALUE)",
			wantErr: "key missing",
		},
		{
			name:    "Missing closing parenthesis",
			input:   "(FOO=BAR",
			wantErr: "reading closing parens: unexpected end of input",
		},
		{
			name:  "Nested with empty values",
			input: `(NODE=something=with=equals) (KEY= ) (EMPTY=)`,
			expected: []Node{
				{Key: "NODE", Value: "something=with=equals"},
				{Key: "KEY", Value: ""},
				{Key: "EMPTY", Value: ""},
			},
		},
		{
			name: "Nested deep structure",
			input: `(DESCRIPTION=
			(ADDRESS=(PROTOCOL=tcps)(HOST=127.0.0.1)(PORT=54557))
			(CONNECT_DATA=
				(CID=(PROGRAM=SQLcl)(HOST=__jdbc__)(USER=marek))
				(SERVICE_NAME=XE)
				(CONNECTION_ID=MAVsTlvrTyqsibsnisguzw==)
			)
		)`,
			expected: []Node{
				{
					Key:   "DESCRIPTION",
					Value: "",
					Children: []Node{
						{
							Key:   "ADDRESS",
							Value: "",
							Children: []Node{
								{Key: "PROTOCOL", Value: "tcps"},
								{Key: "HOST", Value: "127.0.0.1"},
								{Key: "PORT", Value: "54557"},
							},
						},
						{
							Key:   "CONNECT_DATA",
							Value: "",
							Children: []Node{
								{
									Key:   "CID",
									Value: "",
									Children: []Node{
										{Key: "PROGRAM", Value: "SQLcl"},
										{Key: "HOST", Value: "__jdbc__"},
										{Key: "USER", Value: "marek"},
									},
								},
								{Key: "SERVICE_NAME", Value: "XE"},
								{Key: "CONNECTION_ID", Value: "MAVsTlvrTyqsibsnisguzw=="},
							},
						},
					},
				},
			},
		},
		{
			name:    "Missing start of tree",
			input:   "FOO=BAR)",
			wantErr: "expected ')', got 'F'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ParseNodes(tt.input)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestPath(t *testing.T) {
	const input = `(DESCRIPTION=
			(ADDRESS=(PROTOCOL=tcps)(HOST=127.0.0.1)(PORT=54557))
			(CONNECT_DATA=
				(CID=(PROGRAM=SQLcl)(HOST=__jdbc__)(USER=marek))
				(SERVICE_NAME=XE)
				(CONNECTION_ID=MAVsTlvrTyqsibsnisguzw==)
			)
		)`

	out, err := ParseNodes(input)
	require.Len(t, out, 1)
	require.NoError(t, err)

	node := &Node{Children: out}

	require.Equal(t, node, node.Path())
	require.Nil(t, node.Path("DESCRIPTION", "ADDRESS", "NO_SUCH_KEY"))
	require.Equal(t, node.Path("DESCRIPTION"), &node.Children[0])
	require.Equal(t, "tcps", node.Path("DESCRIPTION", "ADDRESS", "PROTOCOL").GetValue())
	require.Empty(t, node.Path("DUMMY").GetValue())
}

func TestNodeString(t *testing.T) {
	tests := []struct {
		name     string
		node     *Node
		expected string
	}{
		{
			name:     "Simple",
			node:     &Node{Key: "FOO", Value: "BAR"},
			expected: "(FOO=BAR)",
		},
		{
			name:     "Error",
			node:     &Node{Key: "BROKEN", ParseErr: "boom"},
			expected: "(BROKEN=?ERROR? boom)",
		},
		{
			name: "Children",
			node: &Node{
				Key: "PARENT",
				Children: []Node{
					{Key: "CHILD1", Value: "Val1"},
					{Key: "CHILD2", Value: "Val2"},
				},
			},
			expected: "(PARENT=(CHILD1=Val1)(CHILD2=Val2))",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := tt.node.String()
			require.Equal(t, tt.expected, out)
		})
	}
}

func FuzzParseNodes(f *testing.F) {
	seeds := []string{
		"(FOO=BAR)",
		"(KEY=(CHILD=VALUE))",
		"(MISSING_EQUALS)",
		"",
		"(=NO_KEY)",
		"(NESTED=(SUB=1)(SUB2=(INNER=ABC)))",
		"(UNTERMINATED=VALUE",
		"(A= (B= (C=DEEP)))",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, orig string) {
		nodes, err := ParseNodes(orig)
		if err != nil {
			return
		}
		// Combine back
		combined := ""
		for _, n := range nodes {
			combined += n.String()
		}
		// Parse again.
		_, _ = ParseNodes(combined)
	})
}
