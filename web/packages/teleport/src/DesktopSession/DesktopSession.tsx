/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useCallback, useEffect, useState } from 'react';
import { useParams } from 'react-router';

import {
  DisconnectedState,
  DesktopSession as SharedDesktopSession,
} from 'shared/components/DesktopSession';
import { useAsync } from 'shared/hooks/useAsync';
import { BrowserFileSystem, TdpClient } from 'shared/libs/tdp';

import { useTeleport } from 'teleport';
import AuthnDialog from 'teleport/components/AuthnDialog';
import cfg, { UrlDesktopParams } from 'teleport/config';
import { AuthenticatedWebSocket } from 'teleport/lib/AuthenticatedWebSocket';
import { adaptWebSocketToTdpTransport } from 'teleport/lib/tdp';
import { shouldShowMfaPrompt, useMfaEmitter } from 'teleport/lib/useMfa';
import { getHostName } from 'teleport/services/api';

export function DesktopSession() {
  const ctx = useTeleport();
  const { username, desktopName, clusterId } = useParams<UrlDesktopParams>();
  useEffect(() => {
    document.title = `${username} on ${desktopName} • ${clusterId}`;
  }, [clusterId, desktopName, username]);

  const [client] = useState(
    () =>
      //TODO(gzdunek): It doesn't really matter here, but make TdpClient reactive to addr change.
      new TdpClient(
        abortSignal =>
          adaptWebSocketToTdpTransport(
            new AuthenticatedWebSocket(
              cfg.api.desktopWsAddr
                .replace(':fqdn', getHostName())
                .replace(':clusterId', clusterId)
                .replace(':desktopName', desktopName)
                .replace(':username', username)
            ),
            abortSignal
          ),
        new BrowserFileSystem()
      )
  );
  const mfa = useMfaEmitter(client, undefined, {
    // When the user cancels the MFA prompt, shut down the connection.
    // To get a new challenge, we need to recreate it.
    onPromptCancel: useCallback(() => client.shutdown(), [client]),
  });

  const [aclAttempt, fetchAcl] = useAsync(
    useCallback(async () => {
      const { acl } = await ctx.userService.fetchUserContext();
      return acl;
    }, [ctx.userService])
  );

  const hasAnotherSession = useCallback(
    () => ctx.desktopService.checkDesktopIsActive(clusterId, desktopName),
    [clusterId, ctx.desktopService, desktopName]
  );

  useEffect(() => {
    fetchAcl();
  }, [username, clusterId, fetchAcl]);

  return (
    <SharedDesktopSession
      client={client}
      username={username}
      desktop={desktopName}
      customConnectionState={({ retry }) => {
        // Errors, except for dialog cancellations, are handled within the MFA dialog.
        if (mfa.attempt.status === 'error' && !shouldShowMfaPrompt(mfa)) {
          return (
            <DisconnectedState
              message={{
                title: 'This session requires multi factor authentication',
                details: mfa.attempt.statusText,
              }}
              desktopName={desktopName}
              onRetry={() => {
                // Clear the MFA attempt to hide this alert state.
                mfa.reset();
                retry();
              }}
            />
          );
        }
        if (shouldShowMfaPrompt(mfa)) {
          return <AuthnDialog mfaState={mfa} />;
        }
      }}
      aclAttempt={aclAttempt}
      hasAnotherSession={hasAnotherSession}
    />
  );
}
