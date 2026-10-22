/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package githttp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	usercredpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalcredentials/v1"
	"github.com/gravitational/teleport/lib/utils"
)

// upstreamAuth decrypts service-encrypted tokens for authenticating to the
// upstream git provider.
type upstreamAuth struct {
	client AuthClient
	cache  *utils.FnCache
	logger *slog.Logger
}

func newUpstreamAuth(ctx context.Context, clock clockwork.Clock, client AuthClient, logger *slog.Logger) (upstreamAuth, error) {
	cache, err := utils.NewFnCache(utils.FnCacheConfig{
		TTL:         10 * time.Minute,
		Clock:       clock,
		Context:     ctx,
		ReloadOnErr: true,
	})
	if err != nil {
		return upstreamAuth{}, trace.Wrap(err)
	}
	return upstreamAuth{client: client, cache: cache, logger: logger}, nil
}

func (u *upstreamAuth) removeExpiredCache() {
	u.cache.RemoveExpired()
}

type decryptCacheKey struct {
	// certSHA256 binds the cache entry to the exact certificate.
	certSHA256     [sha256.Size]byte
	encryptedToken string
}

// DecryptToken decrypts encrypted token to plaintext for upstream use.
func (u *upstreamAuth) DecryptToken(ctx context.Context, userCertDER []byte, encryptedToken string) (string, error) {
	// A real encrypted credential is well under 1KB; this just keeps an
	// oversized, likely-malformed value from being cached at all.
	if len(encryptedToken) > 4096 {
		return "", trace.BadParameter("encrypted token exceeds 4096 bytes")
	}
	certSHA256 := sha256.Sum256(userCertDER)
	key := decryptCacheKey{certSHA256: certSHA256, encryptedToken: encryptedToken}
	return u.cache.Get(ctx, key, func(ctx context.Context) (string, error) {
		u.logger.DebugContext(ctx, "Decrypting user external credential", "cert_sha256", hex.EncodeToString(certSHA256[:]))

		blob, err := base64.StdEncoding.DecodeString(encryptedToken)
		if err != nil {
			return "", trace.BadParameter("invalid encrypted token: %v", err)
		}
		resp, err := u.client.UserExternalCredentialClient().DecryptUserExternalCredential(ctx, usercredpb.DecryptUserExternalCredentialRequest_builder{
			AuthEncryptedBlob: blob,
			UserCertificate:   userCertDER,
		}.Build())
		if err != nil {
			return "", trace.Wrap(err)
		}
		return string(resp.GetPlaintext()), nil
	})
}
