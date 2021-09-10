package bot

import (
	"testing"

	"github.com/gravitational/teleport/tool/ci/pkg/environment"

	"github.com/stretchr/testify/require"
)

func TestNewBot(t *testing.T) {
	tests := []struct {
		cfg      Config
		checkErr require.ErrorAssertionFunc
		expected *Bot
	}{
		{
			cfg:      Config{Environment: &environment.Environment{}},
			checkErr: require.NoError,
		},
		{
			cfg:      Config{},
			checkErr: require.Error,
		},
	}
	for _, test := range tests {
		_, err := New(test.cfg)
		test.checkErr(t, err)
	}

}
