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
  useRef,
  useState,
} from 'react';
import { useParams } from 'react-router';

import type { NotificationItem } from 'shared/components/Notification';
import useAttempt from 'shared/hooks/useAttemptNext';
import { debounce } from 'shared/utils/highbar';

import cfg, { type UrlDesktopParams } from 'teleport/config';
import { ButtonState, TdpClient } from 'teleport/lib/tdp';
import { BitmapFrame } from 'teleport/lib/tdp/client';
import {
  ClientScreenSpec,
  ClipboardData,
  PngFrame,
  PointerData,
  ScrollAxis,
} from 'teleport/lib/tdp/codec';
import { useMfaEmitter } from 'teleport/lib/useMfa';
import { Sha256Digest } from 'teleport/lib/util';
import { getHostName } from 'teleport/services/api';
import desktopService from 'teleport/services/desktops';
import userService from 'teleport/services/user';

import { KeyboardHandler } from './KeyboardHandler';
import { TopBarHeight } from './TopBar';
import useTdpClientCanvas from './useTdpClientCanvas';

export type TdpConnection = {
  status: '' | 'open' | 'closed';
  receivedFirstFrame?: boolean;
  statusText: string;
};

export default function useDesktopSession() {
  const { attempt: fetchAttempt, run } = useAttempt('processing');
  const latestClipboardDigest = useRef('');
  const encoder = useRef(new TextEncoder());
  const [directorySharingState, setDirectorySharingState] =
    useState<DirectorySharingState>(defaultDirectorySharingState);

  const [clipboardSharingState, setClipboardSharingState] =
    useState<ClipboardSharingState>(defaultClipboardSharingState);

  // tdpConnection tracks the state of the tdpClient's TDP connection
  // - 'processing' at first
  // - 'success' once the first TdpClientEvent.IMAGE_FRAGMENT is seen
  // - 'failed' if a fatal error is encountered, should have a statusText
  // - '' if the connection closed gracefully by the server, should have a statusText
  const [tdpConnection, setTdpConnection] = useState<TdpConnection>({
    status: '',
    statusText: '',
  });

  const keyboardHandler = useRef<KeyboardHandler>(null);

  useEffect(() => {
    keyboardHandler.current = new KeyboardHandler();

    return () => {
      keyboardHandler.current.dispose();
    };
  }, []);

  const tdpClient = useRef<TdpClient>(null);
  const clientCanvasProps = useTdpClientCanvas();
  const mfa = useMfaEmitter(tdpClient.current);

  const sendLocalClipboardToRemote = useCallback(async () => {
    if (await sysClipboardGuard(clipboardSharingState, 'read')) {
      navigator.clipboard.readText().then(text => {
        Sha256Digest(text, encoder.current).then(digest => {
          if (text && digest !== latestClipboardDigest.current) {
            tdpClient.current?.sendClipboardData({
              data: text,
            });
            latestClipboardDigest.current = digest;
          }
        });
      });
    }
  }, [clipboardSharingState]);

  const onMouseDown = useCallback(
    (e: MouseEvent) => {
      if (e.button === 0 || e.button === 1 || e.button === 2) {
        tdpClient.current.sendMouseButton(e.button, ButtonState.DOWN);
      }

      // Opportunistically sync local clipboard to remote while
      // transient user activation is in effect.
      // https://developer.mozilla.org/en-US/docs/Web/API/Clipboard/readText#security
      sendLocalClipboardToRemote();
    },
    [sendLocalClipboardToRemote]
  );

  const onMouseWheelScroll = useCallback((e: WheelEvent) => {
    e.preventDefault();
    // We only support pixel scroll events, not line or page events.
    // https://developer.mozilla.org/en-US/docs/Web/API/WheelEvent/deltaMode
    if (e.deltaMode === WheelEvent.DOM_DELTA_PIXEL) {
      if (e.deltaX) {
        tdpClient.current.sendMouseWheelScroll(
          ScrollAxis.HORIZONTAL,
          -e.deltaX
        );
      }
      if (e.deltaY) {
        tdpClient.current.sendMouseWheelScroll(ScrollAxis.VERTICAL, -e.deltaY);
      }
    }
  }, []);

  const onMouseUp = useCallback((e: MouseEvent) => {
    if (e.button === 0 || e.button === 1 || e.button === 2) {
      tdpClient.current.sendMouseButton(e.button, ButtonState.UP);
    }
  }, []);

  const onMouseMove = useCallback(
    (e: MouseEvent) => {
      const canvas = clientCanvasProps.canvasRef.current;
      if (!tdpClient.current || !canvas) {
        return;
      }
      const rect = canvas.getBoundingClientRect();
      const x = e.clientX - rect.left;
      const y = e.clientY - rect.top;
      tdpClient.current.sendMouseMove(x, y);
    },
    [clientCanvasProps.canvasRef]
  );

  const onKeyDown = useCallback(
    (e: KeyboardEvent) => {
      keyboardHandler.current.handleKeyboardEvent({
        cli: tdpClient.current,
        e,
        state: ButtonState.DOWN,
      });

      // The key codes in the if clause below are those that have been empirically determined not
      // to count as transient activation events. According to the documentation, a keydown for
      // the Esc key and any "shortcut key reserved by the user agent" don't count as activation
      // events: https://developer.mozilla.org/en-US/docs/Web/Security/User_activation.
      if (e.key !== 'Meta' && e.key !== 'Alt' && e.key !== 'Escape') {
        sendLocalClipboardToRemote();
      }
    },
    [sendLocalClipboardToRemote]
  );

  const onKeyUp = useCallback((e: KeyboardEvent) => {
    keyboardHandler.current.handleKeyboardEvent({
      cli: tdpClient.current,
      e,
      state: ButtonState.UP,
    });
  }, []);

  const { username, desktopName, clusterId } = useParams<UrlDesktopParams>();

  const [hostname, setHostname] = useState<string>('');

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

  const [alerts, setAlerts] = useState<NotificationItem[]>([]);
  const onRemoveAlert = (id: string) => {
    setAlerts(prevState => prevState.filter(alert => alert.id !== id));
  };

  const addr = cfg.api.desktopWsAddr
    .replace(':fqdn', getHostName())
    .replace(':clusterId', clusterId)
    .replace(':desktopName', desktopName)
    .replace(':username', username);

  // Default TdpClientEvent.TDP_CLIPBOARD_DATA handler.
  const onClipboardData = useCallback(
    async (clipboardData: ClipboardData) => {
      if (
        clipboardData.data &&
        (await sysClipboardGuard(clipboardSharingState, 'write'))
      ) {
        navigator.clipboard.writeText(clipboardData.data);
        let digest = await Sha256Digest(clipboardData.data, encoder.current);
        latestClipboardDigest.current = digest;
      }
    },
    [clipboardSharingState]
  );

  const onScreenSpec = useCallback(
    (spec: ClientScreenSpec) => {
      clientCanvasProps.syncCanvas(spec);
    },
    [clientCanvasProps]
  );

  // Default TdpClientEvent.TDP_ERROR and TdpClientEvent.CLIENT_ERROR handler
  const onError = useCallback((error: Error) => {
    setDirectorySharingState(defaultDirectorySharingState);
    setClipboardSharingState(defaultClipboardSharingState);
    // should merge this + wsStatus into 1 connection var
    setTdpConnection(prevState => {
      // Sometimes when a connection closes due to an error, we get a cascade of
      // errors. Here we update the status only if it's not already 'failed', so
      // that the first error message (which is usually the most informative) is
      // displayed to the user.
      if (prevState.status !== 'closed') {
        return {
          status: 'closed',
          statusText: error.message || error.toString(),
        };
      }
      return prevState;
    });
  }, []);

  // Default TdpClientEvent.TDP_WARNING and TdpClientEvent.CLIENT_WARNING handler
  const onWarning = useCallback((warning: string) => {
    setAlerts(prevState => {
      return [
        ...prevState,
        {
          content: warning,
          severity: 'warn',
          id: crypto.randomUUID(),
        },
      ];
    });
  }, []);

  // TODO(zmb3): this is not what an info-level alert should do.
  // rename it to something like onGracefulDisconnect
  const onInfo = useCallback((info: string) => {
    setDirectorySharingState(defaultDirectorySharingState);
    setClipboardSharingState(defaultClipboardSharingState);
    setTdpConnection({
      status: 'closed', // gracefully disconnecting
      statusText: info,
    });
  }, []);

  const onWsOpen = useCallback(() => {
    setTdpConnection({ status: 'open', statusText: '' });
  }, []);

  const onWsClose = useCallback((message: string) => {
    setTdpConnection({ status: 'closed', statusText: message });
  }, []);

  // create a closure to enable rendered buffering and return
  // the "listener" from it
  const onBmpFrame = useCallback(() => {
    const canvas = clientCanvasProps.canvasRef.current;
    if (!canvas) {
      return;
    }
    const ctx = canvas.getContext('2d');

    // Buffered rendering logic
    var bmpBuffer: BitmapFrame[] = [];
    const renderBuffer = () => {
      if (bmpBuffer.length) {
        for (let i = 0; i < bmpBuffer.length; i++) {
          const bmpFrame = bmpBuffer[i];
          if (ctx && bmpFrame.image_data.data.length != 0) {
            ctx.putImageData(bmpFrame.image_data, bmpFrame.left, bmpFrame.top);
          }
        }
        bmpBuffer = [];
      }
      requestAnimationFrame(renderBuffer);
    };
    requestAnimationFrame(renderBuffer);

    const pushToBmpBuffer = (bmpFrame: BitmapFrame) => {
      bmpBuffer.push(bmpFrame);
    };
    return pushToBmpBuffer;
  }, [clientCanvasProps.canvasRef]);

  // create a closure to enable rendered buffering and return
  // the "listener" from it
  const onPngFrame = useCallback(() => {
    const canvas = clientCanvasProps.canvasRef.current;
    if (!canvas) {
      return;
    }
    const ctx = canvas.getContext('2d');

    // Buffered rendering logic
    var pngBuffer: PngFrame[] = [];
    const renderBuffer = () => {
      if (pngBuffer.length) {
        for (let i = 0; i < pngBuffer.length; i++) {
          const pngFrame = pngBuffer[i];
          if (ctx) {
            ctx.drawImage(pngFrame.data, pngFrame.left, pngFrame.top);
          }
        }
        pngBuffer = [];
      }
      requestAnimationFrame(renderBuffer);
    };
    requestAnimationFrame(renderBuffer);

    const pushToPngBuffer = (pngFrame: PngFrame) => {
      pngBuffer.push(pngFrame);
    };
    return pushToPngBuffer;
  }, [clientCanvasProps.canvasRef]);

  const onPointer = useCallback(
    (pointer: PointerData) => {
      const canvas = clientCanvasProps.canvasRef.current;
      if (!canvas) {
        return;
      }
      if (typeof pointer.data === 'boolean') {
        canvas.style.cursor = pointer.data ? 'default' : 'none';
        return;
      }
      let cursor = document.createElement('canvas');
      cursor.width = pointer.data.width;
      cursor.height = pointer.data.height;
      cursor
        .getContext('2d', { colorSpace: pointer.data.colorSpace })
        .putImageData(pointer.data, 0, 0);
      if (pointer.data.width > 32 || pointer.data.height > 32) {
        // scale the cursor down to at most 32px - max size fully supported by browsers
        const resized = document.createElement('canvas');
        let scale = Math.min(32 / cursor.width, 32 / cursor.height);
        resized.width = cursor.width * scale;
        resized.height = cursor.height * scale;

        let context = resized.getContext('2d', {
          colorSpace: pointer.data.colorSpace,
        });
        context.scale(scale, scale);
        context.drawImage(cursor, 0, 0);
        cursor = resized;
      }
      canvas.style.cursor = `url(${cursor.toDataURL()}) ${
        pointer.hotspot_x
      } ${pointer.hotspot_y}, auto`;
    },
    [clientCanvasProps.canvasRef]
  );

  useEffect(() => {
    if (!tdpClient.current) {
      tdpClient.current = new TdpClient(addr, {
        onClipboardData,
        onError,
        onWarning,
        onInfo,
        onWsOpen,
        onPngFrame: onPngFrame(), // for buffered rendering
        onBmpFrame: onBmpFrame(),
        onScreenSpec,
        onPointer,
        onWsClose,
      });
    }
    // because onClipboardData requires information about clipboardSharingState
    // we need to rebind the event handler when the callback is updated
    tdpClient.current.setEventHandler('onClipboardData', onClipboardData);
  }, [
    addr,
    onClipboardData,
    onBmpFrame,
    onPngFrame,
    onError,
    onWarning,
    onInfo,
    onWsOpen,
    onScreenSpec,
    onPointer,
    onWsClose,
  ]);

  const onDisconnect = () => {
    setClipboardSharingState(prevState => ({
      ...prevState,
      isSharing: false,
    }));
    setDirectorySharingState(prevState => ({
      ...prevState,
      isSharing: false,
    }));
    tdpClient.current.shutdown();
  };

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
          tdpClient.current.addSharedDirectory(sharedDirHandle);
          tdpClient.current.sendSharedDirectoryAnnounce();
        })
        .catch(e => {
          setDirectorySharingState(prevState => ({
            ...prevState,
            directorySelected: false,
          }));
          setAlerts(prevState => [
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
        directorySelected: false,
      }));
      setAlerts(prevState => [
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

  const onCtrlAltDel = useCallback(() => {
    if (!tdpClient) {
      return;
    }
    tdpClient.current.sendKeyboardInput('ControlLeft', ButtonState.DOWN);
    tdpClient.current.sendKeyboardInput('AltLeft', ButtonState.DOWN);
    tdpClient.current.sendKeyboardInput('Delete', ButtonState.DOWN);
  }, []);

  const windowOnResize = debounce(
    () => {
      const spec = getDisplaySize();
      tdpClient.current.resize(spec);
    },
    250,
    { trailing: true }
  );

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

  function onFocusOut() {
    keyboardHandler.current.onFocusOut();
  }

  return {
    mfa,
    tdpClient,
    onMouseDown,
    onKeyDown,
    onKeyUp,
    onFocusOut,
    onMouseMove,
    onMouseUp,
    onMouseWheelScroll,
    username,
    hostname,
    tdpConnection,
    onCtrlAltDel,
    alerts,
    onRemoveAlert,
    onDisconnect,
    clipboardSharingState,
    directorySharingState,
    clientCanvasProps,
    fetchAttempt,
    windowOnResize,
    onShareDirectory,
    showAnotherSessionActiveDialog,
    setShowAnotherSessionActiveDialog,
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

// Calculates the size (in pixels) of the display.
// Since we want to maximize the display size for the user, this is simply
// the full width of the screen and the full height sans top bar.
export function getDisplaySize() {
  return {
    width: window.innerWidth,
    height: window.innerHeight - TopBarHeight,
  };
}
