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

import { useMemo, useState } from 'react';

import { Text } from 'design';
import { ACL } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
import { DesktopSession } from 'shared/components/DesktopSession';
import {
  Attempt,
  makeProcessingAttempt,
  makeSuccessAttempt,
} from 'shared/hooks/useAsync';
import { BrowserFileSystem, TdpClient } from 'shared/libs/tdp';
import { TdpTransport } from 'shared/libs/tdp/client';

import Logger from 'teleterm/logger';
import { cloneAbortSignal, TshdClient } from 'teleterm/services/tshd';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import Document from 'teleterm/ui/Document';
import { useWorkspaceLoggedInUser } from 'teleterm/ui/hooks/useLoggedInUser';
import { useLogger } from 'teleterm/ui/hooks/useLogger';
import * as types from 'teleterm/ui/services/workspacesService';
import { DesktopUri, isWindowsDesktopUri, routing } from 'teleterm/ui/uri';

// The check for another active session is disabled in Connect:
// 1. This protection was added to the Web UI to prevent a situation where a user could be tricked
// into clicking a link that would DOS another user's active session.
// https://github.com/gravitational/webapps/pull/1297
// 2. Supporting this in Connect would require changes to the Auth Server;
// otherwise, we could only get session trackers the user has access to.
const noOtherSession = () => Promise.resolve(false);

export function DocumentDesktopSession(props: {
  visible: boolean;
  doc: types.DocumentDesktopSession;
}) {
  const logger = useLogger('DocumentDesktopSession');
  const { desktopUri, login, origin } = props.doc;
  const appCtx = useAppContext();
  const loggedInUser = useWorkspaceLoggedInUser();
  const acl = useMemo<Attempt<ACL>>(() => {
    if (!loggedInUser) {
      return makeProcessingAttempt();
    }
    return makeSuccessAttempt(loggedInUser.acl);
  }, [loggedInUser]);

  const [client] = useState(
    () =>
      new TdpClient(async abortSignal => {
        const stream = appCtx.tshd.connectToDesktop({
          abort: cloneAbortSignal(abortSignal),
        });
        appCtx.usageService.captureProtocolUse({
          uri: desktopUri,
          protocol: 'desktop',
          origin,
          accessThrough: 'proxy_service',
        });
        return adaptGRPCStreamToTdpTransport(
          stream,
          {
            desktopUri,
            login,
          },
          logger
        );
      }, new BrowserFileSystem())
  );

  let content = (
    <Text m="auto" mt={10} textAlign="center">
      Cannot open a connection to "{desktopUri}".
      <br />
      Only Windows desktops are supported.
    </Text>
  );

  if (isWindowsDesktopUri(desktopUri)) {
    content = (
      <DesktopSession
        hasAnotherSession={noOtherSession}
        desktop={
          routing.parseWindowsDesktopUri(desktopUri).params?.windowsDesktopId
        }
        client={client}
        username={login}
        aclAttempt={acl}
      />
    );
  }

  return <Document visible={props.visible}>{content}</Document>;
}

async function adaptGRPCStreamToTdpTransport(
  stream: ReturnType<TshdClient['connectToDesktop']>,
  targetDesktop: {
    desktopUri: DesktopUri;
    login: string;
  },
  logger: Logger
): Promise<TdpTransport> {
  await stream.requests.send({
    targetDesktop,
    data: new Uint8Array(),
  });

  return {
    onMessage: callback =>
      stream.responses.onMessage(message => {
        callback(
          message.data.buffer.slice(
            message.data.byteOffset,
            message.data.byteOffset + message.data.byteLength
          )
        );
      }),
    onError: (...args) => stream.responses.onError(...args),
    onComplete: (...args) => stream.responses.onComplete(...args),
    send: data => {
      // Strings are sent only in the session player.
      if (typeof data === 'string') {
        logger.error('Sending string data is not supported, this is a bug.');
        return;
      }
      return stream.requests.send({
        data: new Uint8Array(data),
      });
    },
  };
}
