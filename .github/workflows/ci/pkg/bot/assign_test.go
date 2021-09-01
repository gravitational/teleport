package bot

import (
	"testing"

	"github.com/google/go-github/v37/github"
	"github.com/stretchr/testify/require"
)

func TestAssign(t *testing.T) {
	username := "reviewer0"
	ghUsers := []*github.User{
		{Login: &username},
	}
	required := []string{"reviewer0"}
	b := &Bot{}

	err := b.assign(required, ghUsers)
	require.NoError(t, err)
	required = append(required, "reviewer1")
	err = b.assign(required, ghUsers)
	require.Error(t, err)
}
