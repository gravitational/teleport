package host

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"os/user"

	"github.com/gravitational/trace"
)

func MaybeSetCommandCredentialAsUser(ctx context.Context, cmd *exec.Cmd, requestUser *user.User, logger *slog.Logger) error {
	return trace.NotImplemented("windows not yet implemented")
}

// UserOpts allow for customizing the resulting command for adding a new user.
type UserOpts struct {
	// UID a user should be created with. When empty, the UID is determined by the
	// useradd command.
	UID string
	// GID a user should be assigned to on creation. When empty, a group of the same name
	// as the user will be used.
	GID string
	// Home directory for a user. When empty, this will be the root directory to match
	// OpenSSH behavior.
	Home string
	// Shell that the user should use when logging in. When empty, the default shell
	// for the host is used (typically /usr/bin/sh).
	Shell string
}

var ErrInvalidSudoers = errors.New("visudo: invalid sudoers file")
