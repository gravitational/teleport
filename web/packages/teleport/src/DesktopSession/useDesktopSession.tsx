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
import { useClipboardReadWrite } from './useClipboard';
import useWebAuthn from 'teleport/lib/useWebAuthn';
import { UrlDesktopParams } from 'teleport/config';
import desktopService from 'teleport/services/desktops';
import userService from 'teleport/services/user';
import useTdpClientCanvas from './useTdpClientCanvas';

export default function useDesktopSession() {
  const { attempt: fetchAttempt, run } = useAttempt('processing');

  // tdpConnection tracks the state of the tdpClient's TDP connection
  // tdpConnection.status ===
  // - 'processing' at first
  // - 'success' once the first TdpClientEvent.IMAGE_FRAGMENT is seen
  // - 'failed' if a TdpClientEvent.TDP_ERROR is encountered
  const { attempt: tdpConnection, setAttempt: setTdpConnection } =
    useAttempt('processing');

  // wsConnection track's the state of the tdpClient's websocket connection.
  // 'closed' to start, 'open' when TdpClientEvent.WS_OPEN is encountered, then 'closed'
  // again when TdpClientEvent.WS_CLOSE is encountered.
  const [wsConnection, setWsConnection] = useState<'open' | 'closed'>('closed');

  // disconnected tracks whether the user intentionally disconnected the client
  const [disconnected, setDisconnected] = useState(false);

  // recording tracks whether or not a recording is in progress
  const [isRecording, setIsRecording] = useState(false);

  const { username, desktopName, clusterId } = useParams<UrlDesktopParams>();

  const [hostname, setHostname] = useState<string>('');

  const isUsingChrome = navigator.userAgent.includes('Chrome');
  // hasClipboardSharingEnabled tracks whether the acl grants this user
  // clipboard sharing permissions (based on the user's RBAC settings).
  const [hasClipboardSharingEnabled, setHasClipboardSharingEnabled] =
    useState(false);
  // clipboardRWPermission tracks the browser's clipboard api permission.
  const clipboardRWPermission = useClipboardReadWrite(
    isUsingChrome && hasClipboardSharingEnabled
  );

  // clipboardState tracks the overall clipboard state for the component,
  // based on `isUsingChrome`, `hasClipboardSharingEnabled` (whether user's RBAC grants
  // them permission to share their clipboard), and `clipboardRWPermission` (the known
  // state of the browser's clipboard permission).
  const [clipboardState, setClipboardState] = useState({
    enabled: hasClipboardSharingEnabled, // tracks whether the acl grants this user clipboard sharing permissions
    permission: clipboardRWPermission, // tracks the browser clipboard api permission
    errorText: '', // empty string means no error
  });
  useEffect(() => {
    // errors:
    // - browser clipboard permissions error
    // - RBAC permits, browser not chromium
    // - RBAC permits, browser clipboard permissions denied
    if (clipboardRWPermission.state === 'error') {
      setClipboardState({
        enabled: hasClipboardSharingEnabled,
        permission: clipboardRWPermission,
        errorText:
          clipboardRWPermission.errorText ||
          'unknown clipboard permission error',
      });
    } else if (hasClipboardSharingEnabled && !isUsingChrome) {
      setClipboardState({
        enabled: hasClipboardSharingEnabled,
        permission: clipboardRWPermission,
        errorText:
          'Your user role supports clipboard sharing over desktop access, \
          however this feature is only available on chromium based browsers \
          like Brave or Google Chrome. Please switch to a supported browser.',
      });
    } else if (
      hasClipboardSharingEnabled &&
      clipboardRWPermission.state === 'denied'
    ) {
      setClipboardState({
        enabled: hasClipboardSharingEnabled,
        permission: clipboardRWPermission,
        errorText: `Your user role supports clipboard sharing over desktop access, \
        but your browser is blocking clipboard read or clipboard write permissions. \
        Please grant both of these permissions to Teleport in your browser's settings.`,
      });
    } else {
      setClipboardState({
        enabled: hasClipboardSharingEnabled,
        permission: clipboardRWPermission,
        errorText: '',
      });
    }
  }, [isUsingChrome, hasClipboardSharingEnabled, clipboardRWPermission]);

  document.title = useMemo(
    () => `${clusterId} • ${username}@${hostname}`,
    [hostname]
  );

  useEffect(() => {
    run(() =>
      Promise.all([
        desktopService
          .fetchDesktop(clusterId, desktopName)
          .then(desktop => setHostname(desktop.name)),
        userService.fetchUserContext().then(user => {
          setHasClipboardSharingEnabled(user.acl.clipboardSharingEnabled);
          setIsRecording(user.acl.desktopSessionRecordingEnabled);
        }),
      ])
    );
  }, [clusterId, desktopName]);

  const clientCanvasProps = useTdpClientCanvas({
    username,
    desktopName,
    clusterId,
    setTdpConnection,
    setWsConnection,
    enableClipboardSharing:
      clipboardState.enabled &&
      clipboardState.permission.state === 'granted' &&
      !clipboardState.errorText,
  });

  const webauthn = useWebAuthn(clientCanvasProps.tdpClient);

  return {
    hostname,
    username,
    clipboardState,
    isRecording,
    fetchAttempt,
    tdpConnection,
    wsConnection,
    disconnected,
    setDisconnected,
    webauthn,
    ...clientCanvasProps,
  };
}

export type State = ReturnType<typeof useDesktopSession>;
