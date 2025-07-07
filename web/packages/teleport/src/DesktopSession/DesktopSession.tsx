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

import { DesktopSession as SharedDesktopSession } from 'shared/components/DesktopSession';
import { useAsync } from 'shared/hooks/useAsync';
import { TdpClient } from 'shared/libs/tdp';

import { useTeleport } from 'teleport';
import AuthnDialog from 'teleport/components/AuthnDialog';
import cfg, { UrlDesktopParams } from 'teleport/config';
import { AuthenticatedWebSocket } from 'teleport/lib/AuthenticatedWebSocket';
import { adaptWebSocketToTdpTransport } from 'teleport/lib/tdp';
import useWebAuthn, { WebAuthnState } from 'teleport/lib/useWebAuthn';
import { getHostName } from 'teleport/services/api';
import auth from 'teleport/services/auth';

export function DesktopSession() {
  const ctx = useTeleport();
  const { username, desktopName, clusterId } = useParams<UrlDesktopParams>();
  useEffect(() => {
    document.title = `${username} on ${desktopName} • ${clusterId}`;
  }, [clusterId, desktopName, username]);

  const [client] = useState(
    () =>
      //TODO(gzdunek): It doesn't really matter here, but make TdpClient reactive to addr change.
      new TdpClient(abortSignal =>
        adaptWebSocketToTdpTransport(
          new AuthenticatedWebSocket(
            cfg.api.desktopWsAddr
              .replace(':fqdn', getHostName())
              .replace(':clusterId', clusterId)
              .replace(':desktopName', desktopName)
              .replace(':username', username)
          ),
          abortSignal
        )
      )
  );
  const webauthn = useWebAuthn(client);

  const [aclAttempt, fetchAcl] = useAsync(
    useCallback(async () => {
      const { acl } = await ctx.userService.fetchUserContext();
      return acl;
    }, [ctx.userService])
  );

  // Returns an active session only if per-session MFA is disabled.
  // This improves the user experience by preventing multiple confirmation prompts:
  // - one from the active desktop alert,
  // - another from the per-session MFA prompt.
  // The check for another session was added to prevent a situation where a user could be tricked
  // into clicking a link that would DOS another user's active session.
  // https://github.com/gravitational/webapps/pull/1297
  // Showing only the MFA prompt is enough for security.
  const hasAnotherSession = useCallback(async (): Promise<boolean> => {
    const [mfaRequiredResponse, desktopActive] = await Promise.all([
      auth.checkMfaRequired(clusterId, {
        windows_desktop: {
          desktop_name: desktopName,
          login: username,
        },
      }),
      ctx.desktopService.checkDesktopIsActive(clusterId, desktopName),
    ]);
    if (mfaRequiredResponse.required) {
      return false;
    }
    return desktopActive;
  }, [clusterId, ctx.desktopService, desktopName, username]);

  useEffect(() => {
    fetchAcl();
  }, [username, clusterId, fetchAcl]);

  return (
    <SharedDesktopSession
      client={client}
      username={username}
      desktop={desktopName}
      customConnectionState={() => {
        if (webauthn.requested) {
          return <MfaDialog webauthn={webauthn} />;
        }
      }}
      aclAttempt={aclAttempt}
      browserSupportsSharing={navigator.userAgent.includes('Chrome')}
      hasAnotherSession={hasAnotherSession}
    />
  );
}

const MfaDialog = ({ webauthn }: { webauthn: WebAuthnState }) => {
  return (
    <AuthnDialog
      onContinue={webauthn.authenticate}
      onCancel={() => {
        webauthn.setState(prevState => {
          return {
            ...prevState,
            errorText:
              'This session requires multi factor authentication to continue. Please hit "Retry" and follow the prompts given by your browser to complete authentication.',
          };
        });
      }}
      errorText={webauthn.errorText}
    />
  );
};
