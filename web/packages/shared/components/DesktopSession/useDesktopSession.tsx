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
  useRef,
  useState,
} from 'react';

import type { NotificationItem } from 'shared/components/Notification';
import { Attempt } from 'shared/hooks/useAsync';
import { ClipboardData, TdpClient } from 'shared/libs/tdp';

declare global {
  interface Window {
    showDirectoryPicker: () => Promise<FileSystemDirectoryHandle>;
  }
}

export default function useDesktopSession(
  tdpClient: TdpClient,
  aclAttempt: Attempt<{
    clipboardSharingEnabled: boolean;
    directorySharingEnabled: boolean;
  }>,
  browserSupportsSharing: boolean
) {
  const encoder = useRef(new TextEncoder());
  const latestClipboardDigest = useRef('');
  const [directorySharingState, setDirectorySharingState] = useState<{
    directorySelected: boolean;
  }>({ directorySelected: false });

  const [clipboardSharingState, setClipboardSharingState] = useState<{
    readState?: PermissionState;
    writeState?: PermissionState;
  }>({});

  const clipboardSharing: ClipboardSharingState = {
    ...clipboardSharingState,
    browserSupported: browserSupportsSharing,
    allowedByAcl:
      aclAttempt.status === 'success' &&
      aclAttempt.data.clipboardSharingEnabled,
  };
  const directorySharing: DirectorySharingState = {
    ...directorySharingState,
    browserSupported: browserSupportsSharing,
    allowedByAcl:
      aclAttempt.status === 'success' &&
      aclAttempt.data.directorySharingEnabled,
  };

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

  const [alerts, setAlerts] = useState<NotificationItem[]>([]);
  const onRemoveAlert = (id: string) => {
    setAlerts(prevState => prevState.filter(alert => alert.id !== id));
  };
  const addAlert = useCallback((alert: Omit<NotificationItem, 'id'>) => {
    setAlerts(prevState => [
      ...prevState,
      { ...alert, id: crypto.randomUUID() },
    ]);
  }, []);

  async function sendLocalClipboardToRemote() {
    if (!(await sysClipboardGuard(clipboardSharing, 'read'))) {
      return;
    }
    const text = await navigator.clipboard.readText();
    const digest = await sha256Digest(text, encoder.current);
    if (text && digest !== latestClipboardDigest.current) {
      tdpClient.sendClipboardData({
        data: text,
      });
      latestClipboardDigest.current = digest;
    }
  }

  async function onClipboardData(clipboardData: ClipboardData) {
    if (
      clipboardData.data &&
      (await sysClipboardGuard(clipboardSharing, 'write'))
    ) {
      await navigator.clipboard.writeText(clipboardData.data);
      latestClipboardDigest.current = await sha256Digest(
        clipboardData.data,
        encoder.current
      );
    }
  }

  const onShareDirectory = async () => {
    try {
      await tdpClient.shareDirectory();
      setDirectorySharingState({
        directorySelected: true,
      });
    } catch (e) {
      setDirectorySharingState({
        directorySelected: false,
      });
      addAlert({
        severity: 'warn',
        content: {
          title: 'Could not share a directory',
          description: e.message,
        },
      });
    }
  };

  /** Clears sharing state. */
  const clearSharing = useCallback(() => {
    setDirectorySharingState({
      directorySelected: false,
    });
  }, []);

  return {
    clipboardSharingState: clipboardSharing,
    directorySharingState: directorySharing,
    clearSharing,
    onShareDirectory,
    alerts,
    onRemoveAlert,
    addAlert,
    sendLocalClipboardToRemote,
    onClipboardData,
  };
}

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

/**
 * To be called before any system clipboard read/write operation.
 */
async function sysClipboardGuard(
  clipboardSharingState: ClipboardSharingState,
  checkingFor: 'read' | 'write'
): Promise<boolean> {
  // If we're not allowed to share the clipboard according to the acl
  // or due to the browser we're using, never try to read or write.
  if (!clipboardSharingPossible(clipboardSharingState)) {
    return false;
  }

  // If the relevant state is 'prompt', try the operation so that the
  // user is prompted to allow it.
  const checkingForRead = checkingFor === 'read';
  const checkingForWrite = checkingFor === 'write';
  const relevantStateIsPrompt =
    (checkingForRead && clipboardSharingState.readState === 'prompt') ||
    (checkingForWrite && clipboardSharingState.writeState === 'prompt');
  if (relevantStateIsPrompt) {
    return true;
  }

  // Otherwise try only if both read and write permissions are granted
  // and the document has focus (without focus we get an uncatchable error).
  //
  // Note that there's no situation where only one of read or write is granted,
  // but the other is denied, and we want to try the operation. The feature is
  // either fully enabled or fully disabled.
  return isSharingClipboard(clipboardSharingState) && document.hasFocus();
}

// Adapted from https://developer.mozilla.org/en-US/docs/Web/API/SubtleCrypto/digest#converting_a_digest_to_a_hex_string
async function sha256Digest(
  message: string,
  encoder: TextEncoder = new TextEncoder()
): Promise<string> {
  const msgUint8 = encoder.encode(message); // encode as (utf-8) Uint8Array
  const hashBuffer = await crypto.subtle.digest('SHA-256', msgUint8); // hash the message
  const hashArray = Array.from(new Uint8Array(hashBuffer)); // convert buffer to byte array
  const hashHex = hashArray.map(b => b.toString(16).padStart(2, '0')).join(''); // convert bytes to hex string
  return hashHex;
}
