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

package common

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/userexternalsecrets/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/cryptoutils"
)

type credCommands struct {
	sync *credSyncCommand
}

func newCredCommands(app *kingpin.Application) credCommands {
	cred := app.Command("cred", "Credential management commands.").Hidden()
	return credCommands{
		sync: newCredSyncCommand(cred),
	}
}

type credSyncCommand struct {
	*kingpin.CmdClause
	targetEncryptionKeyID string
	delegationID          string
	beam                  string
	watch                 bool
	timeout               time.Duration
}

func newCredSyncCommand(parent *kingpin.CmdClause) *credSyncCommand {
	cmd := &credSyncCommand{
		CmdClause: parent.Command("sync", "Sync encrypted credentials to other sessions."),
	}
	cmd.Flag("key-id", "Encryption key ID of a specific target session.").
		StringVar(&cmd.targetEncryptionKeyID)
	cmd.Flag("delegation-id", "Delegation session ID to sync all bot instances for.").
		StringVar(&cmd.delegationID)
	cmd.Flag("beam", "Beam name or UUID to sync credentials to.").
		StringVar(&cmd.beam)
	cmd.Flag("watch", "Watch for sessions and auto-sync credentials.").
		BoolVar(&cmd.watch)
	cmd.Flag("timeout", "Maximum time to watch (0 means until ctrl-c, only with --watch).").
		Default("0s").
		DurationVar(&cmd.timeout)
	return cmd
}

func (c *credSyncCommand) run(cf *CLIConf) error {
	switch {
	case c.watch:
		return c.runWatch(cf)
	case c.targetEncryptionKeyID != "":
		return c.runTargetByKeyID(cf)
	case c.delegationID != "":
		return c.runTargetByDelegation(cf, c.delegationID)
	case c.beam != "":
		return c.runTargetByBeam(cf)
	default:
		return trace.BadParameter("specify --key-id, --delegation-id, --beam, or --watch")
	}
}

func (c *credSyncCommand) runTargetByKeyID(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	helper, err := getEncryptionHelper(cf.Context, tc)
	if err != nil {
		return trace.Wrap(err, "no encryption helper available")
	}

	synced, err := syncCredentialsToSession(cf, tc, helper, c.targetEncryptionKeyID)
	if err != nil {
		return trace.Wrap(err)
	}
	if synced == 0 {
		fmt.Fprintln(cf.Stdout(), "Session is up to date.")
	} else {
		fmt.Fprintf(cf.Stdout(), "Synced %d credential(s) to %s\n", synced, c.targetEncryptionKeyID)
	}
	return nil
}

func (c *credSyncCommand) runTargetByBeam(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	var delegationID string
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		rootClient, err := clusterClient.ConnectToRootCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer rootClient.Close()

		beam, err := getBeam(cf.Context, rootClient, c.beam)
		if err != nil {
			return trace.Wrap(err)
		}
		delegationID = beam.GetStatus().GetDelegationSessionId()
		if delegationID == "" {
			return trace.BadParameter("beam %q does not have a delegation session", c.beam)
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return c.runTargetByDelegation(cf, delegationID)
}

func (c *credSyncCommand) runTargetByDelegation(cf *CLIConf, delegationID string) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	helper, err := getEncryptionHelper(cf.Context, tc)
	if err != nil {
		return trace.Wrap(err, "no encryption helper available")
	}

	var totalSynced int
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		// Use the watcher to find sessions matching the delegation ID.
		filter := types.UserSessionCredentialsFilter{User: tc.Username}
		watcher, err := clusterClient.AuthClient.NewWatcher(cf.Context, types.Watch{
			Kinds: []types.WatchKind{{
				Kind:   types.KindUserSessionCredentials,
				Filter: filter.IntoMap(),
			}},
		})
		if err != nil {
			return trace.Wrap(err)
		}
		defer watcher.Close()

		select {
		case event := <-watcher.Events():
			if event.Type != types.OpInit {
				return trace.BadParameter("unexpected event type %v", event.Type)
			}
		case <-watcher.Done():
			return trace.ConnectionProblem(watcher.Error(), "watcher closed")
		case <-cf.Context.Done():
			return cf.Context.Err()
		}

		var targetKeyIDs []string
	drain:
		for {
			select {
			case event := <-watcher.Events():
				if event.Type != types.OpPut {
					continue
				}
				unwrapper, ok := event.Resource.(types.Resource153UnwrapperT[*pb.UserSessionCredentials])
				if !ok {
					continue
				}
				creds := unwrapper.UnwrapT()
				if creds.GetSpec().GetEncryptionKey().GetDelegationSessionId() == delegationID {
					targetKeyIDs = append(targetKeyIDs, creds.GetSpec().GetEncryptionKey().GetKeyId())
				}
			default:
				break drain
			}
		}
		watcher.Close()

		if len(targetKeyIDs) == 0 {
			fmt.Fprintf(cf.Stdout(), "No sessions found for delegation %s\n", delegationID)
			return nil
		}

		for _, keyID := range targetKeyIDs {
			synced, err := syncCredentialsToSession(cf, tc, helper, keyID)
			if err != nil {
				fmt.Fprintf(cf.Stderr(), "Failed to sync to %s: %v\n", keyID, err)
				continue
			}
			totalSynced += synced
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if totalSynced == 0 {
		fmt.Fprintln(cf.Stdout(), "All sessions up to date.")
	} else {
		fmt.Fprintf(cf.Stdout(), "Synced %d credential(s) to delegation session %s\n", totalSynced, delegationID)
	}
	return nil
}

func (c *credSyncCommand) runWatch(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	helper, err := getEncryptionHelper(cf.Context, tc)
	if err != nil {
		return trace.Wrap(err, "no encryption helper available")
	}

	profileStatus, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	myKeyID := getEncryptionKeyID(profileStatus)
	if myKeyID == "" {
		return trace.BadParameter("no encryption key ID found in current profile")
	}

	logger.DebugContext(cf.Context, "Starting credential watcher",
		"my_key_id", myKeyID,
	)

	ctx := cf.Context
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(cf.Context, c.timeout)
		defer cancel()
		fmt.Fprintf(cf.Stdout(), "Watching for new sessions (timeout %s)...\n", c.timeout)
	} else {
		fmt.Fprintln(cf.Stdout(), "Watching for new sessions (ctrl-c to stop)...")
	}

	syncedKeys := make(map[string]struct{})

	return client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		filter := types.UserSessionCredentialsFilter{User: tc.Username}
		watcher, err := clusterClient.AuthClient.NewWatcher(ctx, types.Watch{
			Kinds: []types.WatchKind{{
				Kind:   types.KindUserSessionCredentials,
				Filter: filter.IntoMap(),
			}},
		})
		if err != nil {
			return trace.Wrap(err)
		}
		defer watcher.Close()

		for {
			select {
			case event := <-watcher.Events():
				if event.Type != types.OpPut {
					continue
				}
				unwrapper, ok := event.Resource.(types.Resource153UnwrapperT[*pb.UserSessionCredentials])
				if !ok {
					continue
				}
				creds := unwrapper.UnwrapT()
				keyID := creds.GetSpec().GetEncryptionKey().GetKeyId()
				if keyID == myKeyID {
					continue
				}
				if _, already := syncedKeys[keyID]; already {
					continue
				}
				fmt.Fprintf(cf.Stdout(), "New session detected: %s\n", keyID)
				synced, syncErr := syncCredentialsToSession(cf, tc, helper, keyID)
				if syncErr != nil {
					fmt.Fprintf(cf.Stderr(), "Failed to sync to %s: %v\n", keyID, syncErr)
				} else {
					fmt.Fprintf(cf.Stdout(), "Synced %d credential(s) to %s\n", synced, keyID)
				}
				syncedKeys[keyID] = struct{}{}
			case <-watcher.Done():
				if ctx.Err() != nil {
					fmt.Fprintln(cf.Stdout(), "Watch timeout reached.")
					return nil
				}
				return trace.ConnectionProblem(watcher.Error(), "watcher closed")
			case <-ctx.Done():
				fmt.Fprintln(cf.Stdout(), "Watch stopped.")
				return nil
			}
		}
	})
}

// syncCredentialsToSession reads the caller's session and the target session,
// diffs the credential lists, re-encrypts missing credentials for the target's
// public key, and writes the updated target resource back.
func syncCredentialsToSession(cf *CLIConf, tc *client.TeleportClient, helper encryptionHelper, targetKeyID string) (int, error) {
	var synced int
	err := client.RetryWithRelogin(cf.Context, tc, func() error {
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		secretClient := clusterClient.AuthClient.UserExternalSecretClient()

		// Get caller's session credentials.
		myResp, err := secretClient.GetUserSessionCredentials(cf.Context,
			pb.GetUserSessionCredentialsRequest_builder{}.Build())
		if err != nil {
			return trace.Wrap(err, "getting own credentials")
		}
		myCreds := myResp.GetCredentials().GetSpec().GetCredentials()
		if len(myCreds) == 0 {
			logger.DebugContext(cf.Context, "No credentials to sync")
			return nil
		}

		// Get target session credentials.
		targetResp, err := secretClient.GetUserSessionCredentials(cf.Context,
			pb.GetUserSessionCredentialsRequest_builder{
				EncryptionKeyId: targetKeyID,
			}.Build())
		if err != nil {
			return trace.Wrap(err, "getting target credentials")
		}
		targetResource := targetResp.GetCredentials()
		targetCreds := targetResource.GetSpec().GetCredentials()

		// Build set of existing target credentials.
		targetSet := make(map[string]struct{})
		for _, c := range targetCreds {
			targetSet[c.GetResourceKind()+"/"+c.GetResourceName()] = struct{}{}
		}

		// Parse target's public key for ECIES encryption.
		targetPubKeyDER := targetResource.GetSpec().GetEncryptionKey().GetPublicKey()
		pubKey, err := x509.ParsePKIXPublicKey(targetPubKeyDER)
		if err != nil {
			return trace.Wrap(err, "parsing target public key")
		}
		ecdsaPub, ok := pubKey.(*ecdsa.PublicKey)
		if !ok {
			return trace.BadParameter("target encryption key is not ECDSA P-256")
		}

		// Re-encrypt missing credentials for the target.
		for _, cred := range myCreds {
			key := cred.GetResourceKind() + "/" + cred.GetResourceName()
			if _, exists := targetSet[key]; exists {
				continue
			}

			oauth := cred.GetOauth()
			if oauth == nil || len(oauth.GetAccessTokenBlob()) == 0 {
				continue
			}

			// Decrypt our ECIES layer to get the auth-encrypted blob.
			accessKMSBlob, err := helper.decrypt(cf.Context, oauth.GetAccessTokenBlob())
			if err != nil {
				logger.WarnContext(cf.Context, "Failed to decrypt access token for sync",
					"resource", cred.GetResourceName(), "error", err)
				continue
			}

			// Re-encrypt for target.
			accessBlob, err := cryptoutils.ECIESEncrypt(ecdsaPub, accessKMSBlob)
			if err != nil {
				logger.WarnContext(cf.Context, "Failed to encrypt for target",
					"resource", cred.GetResourceName(), "error", err)
				continue
			}

			var refreshBlob []byte
			if len(oauth.GetRefreshTokenBlob()) > 0 {
				refreshKMSBlob, err := helper.decrypt(cf.Context, oauth.GetRefreshTokenBlob())
				if err != nil {
					logger.WarnContext(cf.Context, "Failed to decrypt refresh token for sync",
						"resource", cred.GetResourceName(), "error", err)
				} else {
					refreshBlob, err = cryptoutils.ECIESEncrypt(ecdsaPub, refreshKMSBlob)
					if err != nil {
						logger.WarnContext(cf.Context, "Failed to encrypt refresh for target",
							"resource", cred.GetResourceName(), "error", err)
					}
				}
			}

			targetCreds = append(targetCreds, pb.Credential_builder{
				ResourceKind: cred.GetResourceKind(),
				ResourceName: cred.GetResourceName(),
				Oauth: pb.OAuthSecret_builder{
					AccessTokenBlob:   accessBlob,
					RefreshTokenBlob:  refreshBlob,
					AccessTokenExpiry: oauth.GetAccessTokenExpiry(),
				}.Build(),
			}.Build())
			synced++
		}

		if synced == 0 {
			return nil
		}

		// Write back the updated target resource.
		targetResource.GetSpec().SetCredentials(targetCreds)
		_, err = secretClient.UpdateUserSessionCredentials(cf.Context,
			pb.UpdateUserSessionCredentialsRequest_builder{
				Credentials: targetResource,
			}.Build())
		if err != nil {
			return trace.Wrap(err, "updating target credentials")
		}
		return nil
	})
	return synced, trace.Wrap(err)
}
