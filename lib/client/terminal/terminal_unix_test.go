package terminal

import (
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// fixedDummyFmt is dummy formatter which ignores entry argument in Format() call and returns the same value, always.
type fixedDummyFmt struct {
	bytes []byte
	err   error
}

func (r fixedDummyFmt) Format(entry *logrus.Entry) ([]byte, error) {
	return r.bytes, r.err
}

func Test_addCRFormatter(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		output  string
		wantErr bool
	}{
		{
			name:   "no newlines",
			input:  "foo bar baz",
			output: "foo bar baz",
		},
		{
			name:   "single newline",
			input:  "foo bar baz\n",
			output: "foo bar baz\r\n",
		},
		{
			name:   "multiple newlines",
			input:  "foo\nbar\nbaz\n",
			output: "foo\r\nbar\r\nbaz\r\n",
		},
		{
			name:    "propagate error",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseFmt := fixedDummyFmt{bytes: []byte(tt.input)}
			if tt.wantErr {
				baseFmt.err = fmt.Errorf("dummy")
			}

			r := newCRFormatter(baseFmt)
			actual, err := r.Format(nil)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.output, string(actual))
			}

		})
	}
}
