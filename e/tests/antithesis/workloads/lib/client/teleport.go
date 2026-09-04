package client

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/testenv"
	teleportclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/utils"
)

// TeleportClientConfig contains the workload-specific Teleport client settings.
type TeleportClientConfig struct {
	// Identity is the name of the workload identity under the tbot credentials directory.
	Identity string

	// Host selects a resource by name.
	Host string

	// Labels selects resources by labels.
	Labels map[string]string

	// PredicateExpression selects resources by predicate expression.
	PredicateExpression string

	// HostLogin is the SSH login for node access.
	HostLogin string

	// Stdout receives client stdout. Defaults to [io.Discard].
	Stdout io.Writer

	// Stderr receives client stderr. Defaults to [io.Discard].
	Stderr io.Writer

	// Stdin provides client stdin. Defaults to an empty reader.
	Stdin io.Reader
}

// NewTeleportClient creates a Teleport client for a workload identity and
// returns a cleanup function for its temporary home directory.
func NewTeleportClient(ctx context.Context, cfg TeleportClientConfig) (clt *teleportclient.TeleportClient, cleanup func(), err error) {
	homeDir, err := os.MkdirTemp("", "teleport-client-*")
	if err != nil {
		return nil, nil, trace.Wrap(err, "creating home dir")
	}
	removeHomeDir := func() {
		if err := os.RemoveAll(homeDir); err != nil {
			slog.WarnContext(ctx, "failed to clean up client home",
				"home", homeDir,
				"error", err)
		}
	}

	clt, err = newTeleportClient(homeDir, cfg)
	if err != nil {
		removeHomeDir()
		return nil, nil, trace.Wrap(err, "creating teleport client")
	}

	return clt, removeHomeDir, nil
}

func newTeleportClient(homedir string, cfg TeleportClientConfig) (*teleportclient.TeleportClient, error) {
	if cfg.Identity == "" {
		return nil, trace.BadParameter("identity is required")
	}
	if cfg.Stdout == nil {
		cfg.Stdout = io.Discard
	}
	if cfg.Stderr == nil {
		cfg.Stderr = io.Discard
	}
	if cfg.Stdin == nil {
		cfg.Stdin = strings.NewReader("")
	}

	proxyAddr := testenv.ProxyAddr()
	path := testenv.IdentityPath(cfg.Identity)
	proxyHost, err := utils.Host(proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err, "parsing proxy address")
	}

	keyRing, err := identityfile.KeyRingFromIdentityFile(path, proxyHost, "")
	if err != nil {
		return nil, trace.Wrap(err, "loading identity key ring")
	}

	if keyRing.ClusterName == "" {
		keyRing.ClusterName = testenv.DefaultClusterName
	}

	store := teleportclient.NewFSClientStore(homedir)
	if err := identityfile.LoadIdentityFileIntoClientStore(store, path, proxyAddr, keyRing.ClusterName); err != nil {
		return nil, trace.Wrap(err, "loading identity into client store")
	}

	clt, err := teleportclient.NewClient(&teleportclient.Config{
		Username:            keyRing.Username,
		SiteName:            keyRing.ClusterName,
		WebProxyAddr:        proxyAddr,
		SSHProxyAddr:        proxyAddr,
		Host:                cfg.Host,
		Labels:              cfg.Labels,
		PredicateExpression: cfg.PredicateExpression,
		HostLogin:           cfg.HostLogin,
		NonInteractive:      true,
		ClientStore:         store,
		Stdout:              cfg.Stdout,
		Stderr:              cfg.Stderr,
		Stdin:               cfg.Stdin,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating client")
	}
	return clt, nil
}
