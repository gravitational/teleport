/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { useEffect, useState, useMemo } from 'react';
import { useParams } from 'react-router';

import useAttempt from 'shared/hooks/useAttemptNext';

import useWebAuthn from 'teleport/lib/useWebAuthn';
import desktopService from 'teleport/services/desktops';
import userService from 'teleport/services/user';

import useTdpClientCanvas from './useTdpClientCanvas';

import type { UrlDesktopParams } from 'teleport/config';
import type { NotificationItem } from 'shared/components/Notification';

export default function useDesktopSession() {
  const { attempt: fetchAttempt, run } = useAttempt('processing');

  // tdpConnection tracks the state of the tdpClient's TDP connection
  // - 'processing' at first
  // - 'success' once the first TdpClientEvent.IMAGE_FRAGMENT is seen
  // - 'failed' if a fatal error is encountered
  const { attempt: tdpConnection, setAttempt: setTdpConnection } =
    useAttempt('processing');

  // wsConnection track's the state of the tdpClient's websocket connection.
  // 'closed' to start, 'open' when TdpClientEvent.WS_OPEN is encountered, then 'closed'
  // again when TdpClientEvent.WS_CLOSE is encountered.
  const [wsConnection, setWsConnection] = useState<'open' | 'closed'>('closed');

  // disconnected tracks whether the user intentionally disconnected the client
  const [disconnected, setDisconnected] = useState(false);

  const [directorySharingState, setDirectorySharingState] = useState({
    canShare: false,
    isSharing: false,
  });

  const { username, desktopName, clusterId } = useParams<UrlDesktopParams>();

  const [hostname, setHostname] = useState<string>('');

  const isUsingChrome = navigator.userAgent.includes('Chrome');

  // Set by result of `user.acl.clipboardSharingEnabled && isUsingChrome` below.
  const [clipboardSharingEnabled, setClipboardSharingEnabled] = useState(false);

  const [showAnotherSessionActiveDialog, setShowAnotherSessionActiveDialog] =
    useState(false);

  document.title = useMemo(
    () => `${clusterId} • ${username}@${hostname}`,
    [clusterId, hostname, username]
  );

  useEffect(() => {
    run(() =>
      Promise.all([
        desktopService
          .fetchDesktop(clusterId, desktopName)
          .then(desktop => setHostname(desktop.name)),
        userService.fetchUserContext().then(user => {
          setClipboardSharingEnabled(
            user.acl.clipboardSharingEnabled && isUsingChrome
          );
          setDirectorySharingState(prevState => ({
            ...prevState,
            canShare: user.acl.directorySharingEnabled,
          }));
        }),
        desktopService
          .checkDesktopIsActive(clusterId, desktopName)
          .then(isActive => {
            setShowAnotherSessionActiveDialog(isActive);
          }),
      ])
    );
  }, [clusterId, desktopName, isUsingChrome, run]);

  const [warnings, setWarnings] = useState<NotificationItem[]>([]);
  const onRemoveWarning = (id: string) => {
    setWarnings(prevState => prevState.filter(warning => warning.id !== id));
  };

  const clientCanvasProps = useTdpClientCanvas({
    username,
    desktopName,
    clusterId,
    setTdpConnection,
    setWsConnection,
    setClipboardSharingEnabled,
    setDirectorySharingState,
    clipboardSharingEnabled,
    setWarnings,
  });
  const tdpClient = clientCanvasProps.tdpClient;

  const webauthn = useWebAuthn(tdpClient);

  const onShareDirectory = () => {
    try {
      window
        .showDirectoryPicker()
        .then(sharedDirHandle => {
          setDirectorySharingState(prevState => ({
            ...prevState,
            isSharing: true,
          }));
          tdpClient.addSharedDirectory(sharedDirHandle);
          tdpClient.sendSharedDirectoryAnnounce();
        })
        .catch(e => {
          setDirectorySharingState(prevState => ({
            ...prevState,
            isSharing: false,
          }));
          setWarnings(prevState => [
            ...prevState,
            {
              id: crypto.randomUUID(),
              severity: 'warn',
              content: 'Failed to open the directory picker: ' + e.message,
            },
          ]);
        });
    } catch (e) {
      setDirectorySharingState(prevState => ({
        ...prevState,
        isSharing: false,
      }));
      setWarnings(prevState => [
        ...prevState,
        {
          id: crypto.randomUUID(),
          severity: 'warn',
          // This is a gross error message, but should be infrequent enough that its worth just telling
          // the user the likely problem, while also displaying the error message just in case that's not it.
          // In a perfect world, we could check for which error message this is and display
          // context appropriate directions.
          content:
            'Encountered an error while attempting to share a directory: ' +
            e.message +
            '. \n\nYour user role supports directory sharing over desktop access, \
          however this feature is only available by default on some Chromium \
          based browsers like Google Chrome or Microsoft Edge. Brave users can \
          use the feature by navigating to brave://flags/#file-system-access-api \
          and selecting "Enable". If you\'re not already, please switch to a supported browser.',
        },
      ]);
    }
  };

  return {
    hostname,
    username,
    clipboardSharingEnabled,
    setClipboardSharingEnabled,
    directorySharingState,
    setDirectorySharingState,
    isUsingChrome,
    fetchAttempt,
    tdpConnection,
    wsConnection,
    disconnected,
    setDisconnected,
    webauthn,
    setTdpConnection,
    showAnotherSessionActiveDialog,
    setShowAnotherSessionActiveDialog,
    onShareDirectory,
    warnings,
    onRemoveWarning,
    ...clientCanvasProps,
  };
}

export type State = ReturnType<typeof useDesktopSession>;
