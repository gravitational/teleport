/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import {
  Dispatch,
  SetStateAction,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from 'react';
import { useParams } from 'react-router';

import useAttempt from 'shared/hooks/useAttemptNext';

import { ButtonState } from 'teleport/lib/tdp';
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
  // - 'failed' if a fatal error is encountered, should have a statusText
  // - '' if the connection closed gracefully by the server, should have a statusText
  const { attempt: tdpConnection, setAttempt: setTdpConnection } =
    useAttempt('processing');

  // wsConnection track's the state of the tdpClient's websocket connection.
  // - 'init' to start
  // - 'open' when TdpClientEvent.WS_OPEN is encountered
  // - then 'closed' again when TdpClientEvent.WS_CLOSE is encountered.
  // Once it's 'closed', it should have the message that came with the TdpClientEvent.WS_CLOSE event..
  const [wsConnection, setWsConnection] = useState<WebsocketAttempt>({
    status: 'init',
  });

  const { username, desktopName, clusterId } = useParams<UrlDesktopParams>();

  const [hostname, setHostname] = useState<string>('');

  const [directorySharingState, setDirectorySharingState] =
    useState<DirectorySharingState>(defaultDirectorySharingState);

  const [clipboardSharingState, setClipboardSharingState] =
    useState<ClipboardSharingState>(defaultClipboardSharingState);

  useEffect(() => {
    const clearReadListenerPromise = initClipboardPermissionTracking(
      'clipboard-read',
      setClipboardSharingState
    );
    const clearWriteListenerPromise = initClipboardPermissionTracking(
      'clipboard-write',
      setClipboardSharingState
    );

    return () => {
      clearReadListenerPromise.then(clearReadListener => clearReadListener());
      clearWriteListenerPromise.then(clearWriteListener =>
        clearWriteListener()
      );
    };
  }, []);

  const [showAnotherSessionActiveDialog, setShowAnotherSessionActiveDialog] =
    useState(false);

  document.title = useMemo(
    () => `${username}@${hostname} • ${clusterId}`,
    [clusterId, hostname, username]
  );

  useEffect(() => {
    run(() =>
      Promise.all([
        desktopService
          .fetchDesktop(clusterId, desktopName)
          .then(desktop => setHostname(desktop.name)),
        userService.fetchUserContext().then(user => {
          setClipboardSharingState(prevState => ({
            ...prevState,
            allowedByAcl: user.acl.clipboardSharingEnabled,
          }));
          setDirectorySharingState(prevState => ({
            ...prevState,
            allowedByAcl: user.acl.directorySharingEnabled,
          }));
        }),
        desktopService
          .checkDesktopIsActive(clusterId, desktopName)
          .then(isActive => {
            setShowAnotherSessionActiveDialog(isActive);
          }),
      ])
    );
  }, [clusterId, desktopName, run]);

  const [warnings, setWarnings] = useState<NotificationItem[]>([]);
  const onRemoveWarning = (id: string) => {
    setWarnings(prevState => prevState.filter(warning => warning.id !== id));
  };
  const addAlert = useCallback((alert: Omit<NotificationItem, 'id'>) => {
    setWarnings(prevState => [
      ...prevState,
      { ...alert, id: crypto.randomUUID() },
    ]);
  }, []);

  const clientCanvasProps = useTdpClientCanvas({
    username,
    desktopName,
    clusterId,
    setTdpConnection,
    setWsConnection,
    setClipboardSharingState,
    setDirectorySharingState,
    clipboardSharingState,
    setWarnings,
  });
  const tdpClient = clientCanvasProps.tdpClient;

  const webauthn = useWebAuthn(tdpClient);

  const onShareDirectory = () => {
    try {
      window
        .showDirectoryPicker()
        .then(sharedDirHandle => {
          // Permissions granted and/or directory selected
          setDirectorySharingState(prevState => ({
            ...prevState,
            directorySelected: true,
          }));
          tdpClient.addSharedDirectory(sharedDirHandle);
          tdpClient.sendSharedDirectoryAnnounce();
        })
        .catch(e => {
          setDirectorySharingState(prevState => ({
            ...prevState,
            directorySelected: false,
          }));
          addAlert({
            severity: 'warn',
            content: 'Failed to open the directory picker: ' + e.message,
          });
        });
    } catch (e) {
      setDirectorySharingState(prevState => ({
        ...prevState,
        directorySelected: false,
      }));
      addAlert({
        severity: 'warn',
        // This is a gross error message, but should be infrequent enough that its worth just telling
        // the user the likely problem, while also displaying the error message just in case that's not it.
        // In a perfect world, we could check for which error message this is and display
        // context appropriate directions.
        content: {
          title: 'Encountered an error while attempting to share a directory: ',
          description:
            e.message +
            '. \n\nYour user role supports directory sharing over desktop access, \
  however this feature is only available by default on some Chromium \
  based browsers like Google Chrome or Microsoft Edge. Brave users can \
  use the feature by navigating to brave://flags/#file-system-access-api \
  and selecting "Enable". If you\'re not already, please switch to a supported browser.',
        },
      });
    }
  };

  const onCtrlAltDel = () => {
    if (!tdpClient) {
      return;
    }
    tdpClient.sendKeyboardInput('ControlLeft', ButtonState.DOWN);
    tdpClient.sendKeyboardInput('AltLeft', ButtonState.DOWN);
    tdpClient.sendKeyboardInput('Delete', ButtonState.DOWN);
  };

  return {
    hostname,
    username,
    clipboardSharingState,
    setClipboardSharingState,
    directorySharingState,
    setDirectorySharingState,
    fetchAttempt,
    tdpConnection,
    wsConnection,
    webauthn,
    setTdpConnection,
    showAnotherSessionActiveDialog,
    setShowAnotherSessionActiveDialog,
    onShareDirectory,
    onCtrlAltDel,
    warnings,
    onRemoveWarning,
    addAlert,
    setWsConnection,
    ...clientCanvasProps,
  };
}

export type State = ReturnType<typeof useDesktopSession>;

type CommonFeatureState = {
  /**
   * Whether the feature is allowed by the acl.
   *
   * Undefined if it hasn't been queried yet.
   */
  allowedByAcl?: boolean;
  /**
   * Whether the feature is available in the browser.
   */
  browserSupported: boolean;
};

/**
 * The state of the directory sharing feature.
 */
export type DirectorySharingState = CommonFeatureState & {
  /**
   * Whether the user is currently sharing a directory.
   */
  directorySelected: boolean;
};

/**
 * The state of the clipboard sharing feature.
 */
export type ClipboardSharingState = CommonFeatureState & {
  /**
   * The current state of the 'clipboard-read' permission.
   *
   * Undefined if it hasn't been queried yet.
   */
  readState?: PermissionState;
  /**
   * The current state of the 'clipboard-write' permission.
   *
   * Undefined if it hasn't been queried yet.
   */
  writeState?: PermissionState;
};

export type Setter<T> = Dispatch<SetStateAction<T>>;

async function initClipboardPermissionTracking(
  name: 'clipboard-read' | 'clipboard-write',
  setClipboardSharingState: Setter<ClipboardSharingState>
) {
  const handleChange = () => {
    if (name === 'clipboard-read') {
      setClipboardSharingState(prevState => ({
        ...prevState,
        readState: perm.state,
      }));
    } else {
      setClipboardSharingState(prevState => ({
        ...prevState,
        writeState: perm.state,
      }));
    }
  };

  // Query the permission state
  const perm = await navigator.permissions.query({
    name: name as PermissionName,
  });

  // Set its change handler
  perm.onchange = handleChange;
  // Set the initial state
  handleChange();

  // Return a cleanup function that removes the change handler (for use by useEffect)
  return () => {
    perm.onchange = null;
  };
}

/**
 * Determines whether a feature is/should-be possible based on whether it's allowed by the acl
 * and whether it's supported by the browser.
 */
function commonFeaturePossible(
  commonFeatureState: CommonFeatureState
): boolean {
  return commonFeatureState.allowedByAcl && commonFeatureState.browserSupported;
}

/**
 * Determines whether clipboard sharing is/should-be possible based on whether it's allowed by the acl
 * and whether it's supported by the browser.
 */
export function clipboardSharingPossible(
  clipboardSharingState: ClipboardSharingState
): boolean {
  return commonFeaturePossible(clipboardSharingState);
}

/**
 * Returns whether clipboard sharing is active.
 */
export function isSharingClipboard(
  clipboardSharingState: ClipboardSharingState
): boolean {
  return (
    clipboardSharingState.allowedByAcl &&
    clipboardSharingState.browserSupported &&
    clipboardSharingState.readState === 'granted' &&
    clipboardSharingState.writeState === 'granted'
  );
}

/**
 * Provides a user-friendly message indicating whether clipboard sharing is enabled,
 * and the reason it is disabled.
 */
export function clipboardSharingMessage(state: ClipboardSharingState): string {
  if (!state.allowedByAcl) {
    return 'Clipboard Sharing disabled by Teleport RBAC.';
  }
  if (!state.browserSupported) {
    return 'Clipboard Sharing is not supported in this browser.';
  }
  if (state.readState === 'denied' || state.writeState === 'denied') {
    return 'Clipboard Sharing disabled due to browser permissions.';
  }

  return isSharingClipboard(state)
    ? 'Clipboard Sharing enabled.'
    : 'Clipboard Sharing disabled.';
}

/**
 * Determines whether directory sharing is/should-be possible based on whether it's allowed by the acl
 * and whether it's supported by the browser.
 */
export function directorySharingPossible(
  directorySharingState: DirectorySharingState
): boolean {
  return commonFeaturePossible(directorySharingState);
}

/**
 * Returns whether directory sharing is active.
 */
export function isSharingDirectory(
  directorySharingState: DirectorySharingState
): boolean {
  return (
    directorySharingState.allowedByAcl &&
    directorySharingState.browserSupported &&
    directorySharingState.directorySelected
  );
}

export const defaultDirectorySharingState: DirectorySharingState = {
  browserSupported: navigator.userAgent.includes('Chrome'),
  directorySelected: false,
};

export const defaultClipboardSharingState: ClipboardSharingState = {
  browserSupported: navigator.userAgent.includes('Chrome'),
};

export type WebsocketAttempt = {
  status: 'init' | 'open' | 'closed';
  statusText?: string;
};
