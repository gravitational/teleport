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

import { useCallback, useMemo, useState } from 'react';

import { Text } from 'design';
import { ACL } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
import { DesktopSession } from 'shared/components/DesktopSession';
import {
  Attempt,
  makeProcessingAttempt,
  makeSuccessAttempt,
} from 'shared/hooks/useAsync';
import { SharedDirectoryAccess, TdpClient, useListener } from 'shared/libs/tdp';
import { TdpTransport } from 'shared/libs/tdp/client';

import Logger from 'teleterm/logger';
import { MainProcessClient } from 'teleterm/mainProcess/types';
import { cloneAbortSignal, TshdClient } from 'teleterm/services/tshd';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import Document from 'teleterm/ui/Document';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
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
  const { desktopUri, login, origin, uri } = props.doc;
  const appCtx = useAppContext();
  const { documentsService } = useWorkspaceContext();
  const loggedInUser = useWorkspaceLoggedInUser();
  const acl = useMemo<Attempt<ACL>>(() => {
    if (!loggedInUser?.acl) {
      return makeProcessingAttempt();
    }
    return makeSuccessAttempt(loggedInUser.acl);
  }, [loggedInUser]);

  const [client] = useState(
    () =>
      new TdpClient(
        async abortSignal => {
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
            { desktopUri, login },
            logger
          );
        },
        makeTshdFileSystem(appCtx.mainProcessClient, {
          desktopUri,
          login,
        })
      )
  );

  useListener(
    client.onTransportOpen,
    useCallback(
      () => documentsService.update(uri, { status: 'connected' }),
      [documentsService, uri]
    )
  );
  useListener(
    client.onTransportClose,
    useCallback(
      error => documentsService.update(uri, { status: error ? 'error' : '' }),
      [documentsService, uri]
    )
  );
  useListener(
    client.onError,
    useCallback(
      () => documentsService.update(uri, { status: 'error' }),
      [documentsService, uri]
    )
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
        browserSupportsSharing
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

/**
 * The tshd daemon is responsible for handling directory sharing.
 *
 * The process begins when the Electron main process opens a directory picker.
 * Once a path is selected, it is passed to tshd via the SetSharedDirectoryForDesktopSession API.
 *
 * tshd then verifies whether there is an active session for the specified desktop user and attempts to open the directory.
 * Once that's done, everything is ready on the tsh daemon to intercept and handle the file system events.
 *
 * The final step is to send a SharedDirectoryAnnounce message to the server, which is done in the JS TDP client.
 * This message is safe to send from the renderer because it only provides
 * a display name for the mounted drive on the remote machine and has no effect on local file system operations.
 */
function makeTshdFileSystem(
  mainProcessClient: MainProcessClient,
  target: {
    desktopUri: string;
    login: string;
  }
): SharedDirectoryAccess {
  let directoryName = '';
  return {
    selectDirectory: async () => {
      directoryName =
        await mainProcessClient.selectDirectoryForDesktopSession(target);
    },
    getDirectoryName: () => directoryName,
    // These functions are unimplemented because all file system operations
    // are handled exclusively by the tsh daemon.
    stat: () => {
      throw new NotImplemented();
    },
    readDir: () => {
      throw new NotImplemented();
    },
    read: () => {
      throw new NotImplemented();
    },
    write: () => {
      throw new NotImplemented();
    },
    truncate: () => {
      throw new NotImplemented();
    },
    create: () => {
      throw new NotImplemented();
    },
    delete: () => {
      throw new NotImplemented();
    },
  };
}

class NotImplemented extends Error {
  constructor() {
    super('Not implemented, file system operation are handled by tsh demon.');
  }
}
