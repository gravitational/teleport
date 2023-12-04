package gcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

func TestFoobar(t *testing.T) {
	ctx := context.Background()
	cli, err := sqladmin.NewService(ctx)
	require.NoError(t, err)

	_, err = cli.Users.Update("teleport-dev-320620", "marek-gcp-mysql-01", &sqladmin.User{
		Password: "fdasfdasf",
	}).Name("alice").Host("%").Context(ctx).Do()

	require.NoError(t, err)
}
