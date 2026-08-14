package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/testenv"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/utils"
)

func setupClient(ctx context.Context, params *TestCaseParams, stdout, stderr io.Writer) (clt *client.TeleportClient, cleanup func(), err error) {
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

	clt, err = newTeleportClient(homeDir, params, stdout, stderr)
	if err != nil {
		removeHomeDir()
		return nil, nil, trace.Wrap(err, "creating teleport client")
	}

	return clt, removeHomeDir, nil
}

func newTeleportClient(homedir string, params *TestCaseParams, stdout, stderr io.Writer) (*client.TeleportClient, error) {
	proxyAddr := testenv.ProxyAddr()
	path := testenv.IdentityPath(params.Identity)
	proxyHost, err := utils.Host(proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err, "parsing proxy address")
	}

	keyRing, err := identityfile.KeyRingFromIdentityFile(path, proxyHost, "")
	if err != nil {
		return nil, trace.Wrap(err, "loading identity key ring")
	}

	if keyRing.ClusterName == "" {
		keyRing.ClusterName = defaultClusterName
	}

	store := client.NewFSClientStore(homedir)
	if err := identityfile.LoadIdentityFileIntoClientStore(store, path, proxyAddr, keyRing.ClusterName); err != nil {
		return nil, trace.Wrap(err, "loading identity into client store")
	}

	clt, err := client.NewClient(&client.Config{
		Username:            keyRing.Username,
		SiteName:            keyRing.ClusterName,
		WebProxyAddr:        proxyAddr,
		SSHProxyAddr:        proxyAddr,
		Host:                params.Target.host(),
		Labels:              params.Target.labels(),
		PredicateExpression: params.Target.PredicateExpression,
		HostLogin:           params.HostUser,
		NonInteractive:      true,
		ClientStore:         store,
		Stdout:              stdout,
		Stderr:              stderr,
		Stdin:               strings.NewReader(""),
	})

	if err != nil {
		return nil, trace.Wrap(err, "creating client")
	}
	return clt, nil
}
