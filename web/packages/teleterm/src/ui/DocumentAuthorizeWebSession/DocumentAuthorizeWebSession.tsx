/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Alert, ButtonPrimary, ButtonText, H1, Text } from 'design';
import Flex from 'design/Flex';
import { DeviceConfirmationToken } from 'gen-proto-ts/teleport/devicetrust/v1/device_confirmation_token_pb';
import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
import { Attempt, useAsync } from 'shared/hooks/useAsync';
import { processRedirectUri } from 'shared/redirects';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import Document from 'teleterm/ui/Document';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import * as types from 'teleterm/ui/services/workspacesService';
import { WebSessionRequest } from 'teleterm/ui/services/workspacesService';
import { routing } from 'teleterm/ui/uri';
import { retryWithRelogin } from 'teleterm/ui/utils';

export function DocumentAuthorizeWebSession(props: {
  doc: types.DocumentAuthorizeWebSession;
  visible: boolean;
}) {
  const ctx = useAppContext();
  const { documentsService } = useWorkspaceContext();
  const rootCluster = ctx.clustersService.findCluster(props.doc.rootClusterUri);
  const [authorizeAttempt, authorize] = useAsync(async () => {
    const {
      response: { confirmationToken },
    } = await retryWithRelogin(ctx, props.doc.rootClusterUri, () =>
      ctx.clustersService.authenticateWebDevice(
        props.doc.rootClusterUri,
        props.doc.webSessionRequest
      )
    );
    return confirmationToken;
  });
  const clusterName = routing.parseClusterName(props.doc.rootClusterUri);
  const isDeviceTrusted = rootCluster.loggedInUser?.isDeviceTrusted;
  const isRequestedUserLoggedIn =
    props.doc.webSessionRequest.username === rootCluster.loggedInUser?.name;
  const canAuthorize = isDeviceTrusted && isRequestedUserLoggedIn;

  async function authorizeAndCloseDocument() {
    const [confirmationToken, error] = await authorize();
    if (!error) {
      const url = buildAuthorizedSessionUrl(
        rootCluster,
        props.doc.webSessionRequest,
        confirmationToken
      );
      // This endpoint verifies the token and "upgrades" the web session and redirects to "/web".
      window.open(url);
      closeAndNotify();
    }
  }

  function openUnauthorizedAndCloseDocument() {
    const url = buildUnauthorizedSessionUrl(
      rootCluster,
      props.doc.webSessionRequest
    );
    window.open(url);
    closeAndNotify();
  }

  function closeAndNotify() {
    documentsService.close(props.doc.uri);
    ctx.notificationsService.notifyInfo(
      'Web session has been opened in the browser'
    );
  }

  return (
    <Document visible={props.visible}>
      <Flex
        flexDirection="column"
        maxWidth="680px"
        width="100%"
        mx="auto"
        mt="4"
        px="5"
      >
        <H1 mb="4">Authorize Web Session</H1>
        <Flex flexDirection="column" gap={3}>
          {/*It's technically possible to open a deep link to authorize a session on a device that is not enrolled.*/}
          {!isDeviceTrusted && (
            <Alert
              mb={0}
              details={
                <>
                  To authorize a web session, you must first{' '}
                  <a
                    href="https://goteleport.com/docs/admin-guides/access-controls/device-trust/guide/#step-22-enroll-device"
                    target="_blank"
                  >
                    enroll your device
                  </a>
                  . Then log out of Teleport Connect, log back in, and try
                  again.
                </>
              }
            >
              This device is not trusted
            </Alert>
          )}
          {!isRequestedUserLoggedIn && (
            <Alert
              mb={0}
              primaryAction={{
                content: 'Log Out',
                onClick: () => {
                  ctx.commandLauncher.executeCommand('cluster-logout', {
                    clusterUri: rootCluster.uri,
                  });
                },
              }}
              details={
                <>
                  You are logged in as <b>{rootCluster.loggedInUser?.name}</b>.
                  To authorize this web session request, please log out in
                  Teleport Connect and log in again as{' '}
                  <b>{props.doc.webSessionRequest.username}</b>.
                  <br />
                  Then click Launch Teleport Connect again in the browser.
                </>
              }
            >
              Requested user is not logged in
            </Alert>
          )}
          {authorizeAttempt.status === 'error' && (
            <Alert mb={0} details={authorizeAttempt.statusText}>
              Could not authorize the session
            </Alert>
          )}
          <Text>
            Would you like to authorize a device trust web session for{' '}
            <b>{clusterName}</b>?
            <br />
            The session will automatically open in a new browser tab.
          </Text>
          <Flex flexDirection="column" gap={2}>
            <ButtonPrimary
              disabled={
                !canAuthorize ||
                authorizeAttempt.status === 'processing' ||
                authorizeAttempt.status === 'success'
              }
              size="large"
              onClick={authorizeAndCloseDocument}
            >
              {getButtonText(authorizeAttempt)}
            </ButtonPrimary>
            <ButtonText
              disabled={
                authorizeAttempt.status === 'processing' ||
                authorizeAttempt.status === 'success'
              }
              onClick={openUnauthorizedAndCloseDocument}
            >
              Open Session Without Device Trust
            </ButtonText>
          </Flex>
        </Flex>
      </Flex>
    </Document>
  );
}

const confirmPath = 'webapi/devices/webconfirm';

function buildAuthorizedSessionUrl(
  rootCluster: Cluster,
  webSessionRequest: WebSessionRequest,
  confirmationToken: DeviceConfirmationToken
): string {
  const { redirectUri } = webSessionRequest;

  let url = `https://${rootCluster.proxyHost}/${confirmPath}?id=${confirmationToken.id}&token=${confirmationToken.token}`;
  if (redirectUri) {
    url = `${url}&redirect_uri=${redirectUri}`;
  }
  return url;
}

function buildUnauthorizedSessionUrl(
  rootCluster: Cluster,
  webSessionRequest: WebSessionRequest
): string {
  // processedRedirectUri is the path part of the redirectUri.
  // Unlike in buildAuthorizedSessionUrl, here we return a full path to open
  // instead of including redirection as the `redirect_uri` query parameter.
  const processedRedirectUri = processRedirectUri(
    webSessionRequest.redirectUri
  );
  return `https://${rootCluster.proxyHost}${processedRedirectUri}`;
}

function getButtonText(attempt: Attempt<unknown>): string {
  switch (attempt.status) {
    case '':
    case 'error':
      return 'Authorize Session';
    case 'processing':
      return 'Authorizing Session…';
    case 'success':
      return 'Session Authorized';
  }
}
