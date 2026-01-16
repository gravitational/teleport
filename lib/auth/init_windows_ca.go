// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package auth

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

type migrateWindowsCAParams struct {
	Logger               *slog.Logger
	ClusterConfiguration services.ClusterNameGetter
	Trust                services.Trust
}

// migrateWindowsCA performs the WindowsCA migration, which clones an existing
// UserCA into the new WindowsCA.
//
// TODO(codingllama): DELETE IN 20.
func migrateWindowsCA(ctx context.Context, p migrateWindowsCAParams) error {
	switch {
	case p.Logger == nil:
		return errors.New("param Logger required")
	case p.ClusterConfiguration == nil:
		return errors.New("param ClusterConfiguration required")
	case p.Trust == nil:
		return errors.New("param Trust required")
	}

	cn, err := p.ClusterConfiguration.GetClusterName(ctx)
	if err != nil {
		return trace.Wrap(err, "read cluster name")
	}
	cluster := cn.GetClusterName()

	return trace.Wrap(
		migrateWindowsCAOnCluster(ctx, p.Logger, p.Trust, cluster),
		"migrate cluster %q", cluster,
	)
}

func migrateWindowsCAOnCluster(
	ctx context.Context,
	logger *slog.Logger,
	trust services.Trust,
	clusterName string,
) error {
	dst := types.CertAuthID{Type: types.WindowsCA, DomainName: clusterName}
	src := types.CertAuthID{Type: types.UserCA, DomainName: clusterName}

	logger = logger.With(
		teleport.ComponentKey, "WindowsCA",
		"cluster", clusterName,
		"src", src.Type,
		"dst", dst.Type,
	)

	// Query dst CA.
	switch _, err := trust.GetCertAuthority(ctx, dst, false /* loadSigningKeys */); {
	case err == nil:
		logger.DebugContext(ctx, "Windows CA already exists, nothing to do")
		return nil // dst already exists.
	case !trace.IsNotFound(err):
		return trace.Wrap(err, "read %q CA", dst.Type)
	}

	// Query src CA.
	srcCA, err := trust.GetCertAuthority(ctx, src, true /* loadSigningKeys */)
	switch {
	case trace.IsNotFound(err):
		logger.DebugContext(ctx, "User CA not found (new cluster)")
		return nil // src does not exist, fresh cluster.
	case err != nil:
		return trace.Wrap(err, "read %q CA", src.Type)
	}

	logger.InfoContext(ctx, "Started migration")

	// Verify that the CA holds a private key.
	//
	// If src is undergoing rotation then only the currently active keys are
	// copied. This is by design: the active keys must be trusted by definition
	// and we don't want to create a copied CA that is in a transitional state
	// (ie, mid-rotation).
	activeKeys := srcCA.GetActiveKeys()
	foundViableKey := false
	for _, kp := range activeKeys.TLS {
		if len(kp.Key) > 0 {
			foundViableKey = true
			break
		}
	}
	if !foundViableKey {
		logger.WarnContext(ctx, "ActiveKeys set lacks a viable key, skipping migration")
		return nil
	}

	// Clone!
	dstCA, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        dst.Type,
		ClusterName: clusterName,
		ActiveKeys: types.CAKeySet{
			TLS: activeKeys.TLS,
		},
	})
	if err != nil {
		return trace.Wrap(err, "new cert authority")
	}

	// Create dst CA.
	switch err := trust.CreateCertAuthority(ctx, dstCA); {
	case err == nil:
		logger.InfoContext(ctx, "Migration successful")
		return nil
	case trace.IsAlreadyExists(err):
		logger.InfoContext(ctx, "Migration performed by another Auth instance", "error", err)
		return nil
	default:
		return trace.Wrap(err, "create cert authority")
	}
}
