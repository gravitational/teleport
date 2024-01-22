/* eslint-disable no-console */
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

import { useState, useEffect, useRef, Dispatch, SetStateAction } from 'react';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { NotificationItem } from 'shared/components/Notification';

import { Platform, getPlatform } from 'design/platform';

import { TdpClient, ButtonState, ScrollAxis } from 'teleport/lib/tdp';
import {
  ClientScreenSpec,
  ClipboardData,
  PngFrame,
  SyncKeys,
} from 'teleport/lib/tdp/codec';
import { getAccessToken, getHostName } from 'teleport/services/api';
import cfg from 'teleport/config';
import { Sha256Digest } from 'teleport/lib/util';

import { TopBarHeight } from './TopBar';

import type { BitmapFrame } from 'teleport/lib/tdp/client';

declare global {
  interface Navigator {
    userAgentData?: { platform: any };
  }
}

export default function useTdpClientCanvas(props: Props) {
  const {
    username,
    desktopName,
    clusterId,
    setTdpConnection,
    setWsConnection,
    setClipboardSharingEnabled,
    setDirectorySharingState,
    clipboardSharingEnabled,
    setWarnings,
  } = props;
  const [tdpClient, setTdpClient] = useState<TdpClient | null>(null);
  const initialTdpConnectionSucceeded = useRef(false);
  const encoder = useRef(new TextEncoder());
  const latestClipboardDigest = useRef('');

  /**
   * Tracks whether the next keydown or keyup event should sync the
   * local toggle key state to the remote machine.
   *
   * Set to true:
   * - On component initialization, so keys are synced before the first keydown/keyup event.
   * - On focusout, so keys are synced when the user returns to the window.
   */
  const syncBeforeNextKey = useRef(true);

  useEffect(() => {
    const addr = cfg.api.desktopWsAddr
      .replace(':fqdn', getHostName())
      .replace(':clusterId', clusterId)
      .replace(':desktopName', desktopName)
      .replace(':token', getAccessToken())
      .replace(':username', username);

    setTdpClient(new TdpClient(addr));
  }, [clusterId, username, desktopName]);

  const syncCanvasResolutionAndSize = (canvas: HTMLCanvasElement) => {
    const { width, height } = getDisplaySize();

    // Set a fixed canvas resolution and display size. This ensures
    // that neither of these change when the user resizes the browser
    // window. Instead, the canvas will remain the same size and the
    // browser will add scrollbars if necessary. This is the behavior
    // we want until https://github.com/gravitational/teleport/issues/9702
    // is resolved.
    canvas.width = width;
    canvas.height = height;
    console.debug(`set canvas.width x canvas.height to ${width} x ${height}`);
    canvas.style.width = `${width}px`;
    canvas.style.height = `${height}px`;
    console.debug(
      `set canvas.style.width x canvas.style.height to ${width} x ${height}`
    );
  };

  // Default TdpClientEvent.TDP_PNG_FRAME handler (buffered)
  const clientOnPngFrame = (
    ctx: CanvasRenderingContext2D,
    pngFrame: PngFrame
  ) => {
    // The first image fragment we see signals a successful TDP connection.
    if (!initialTdpConnectionSucceeded.current) {
      syncCanvasResolutionAndSize(ctx.canvas);
      setTdpConnection({ status: 'success' });
      initialTdpConnectionSucceeded.current = true;
    }
    ctx.drawImage(pngFrame.data, pngFrame.left, pngFrame.top);
  };

  // Default TdpClientEvent.TDP_BMP_FRAME handler (buffered)
  const clientOnBitmapFrame = (
    ctx: CanvasRenderingContext2D,
    bmpFrame: BitmapFrame
  ) => {
    // The first image fragment we see signals a successful TDP connection.
    if (!initialTdpConnectionSucceeded.current) {
      syncCanvasResolutionAndSize(ctx.canvas);
      setTdpConnection({ status: 'success' });
      initialTdpConnectionSucceeded.current = true;
    }
    ctx.putImageData(bmpFrame.image_data, bmpFrame.left, bmpFrame.top);
  };

  // Default TdpClientEvent.TDP_CLIENT_SCREEN_SPEC handler.
  const clientOnClientScreenSpec = (
    cli: TdpClient,
    canvas: HTMLCanvasElement,
    spec: ClientScreenSpec
  ) => {
    const { width, height } = spec;
    canvas.width = width;
    canvas.height = height;
    console.debug(`set canvas.width x canvas.height to ${width} x ${height}`);
    canvas.style.width = `${width}px`;
    canvas.style.height = `${height}px`;
    console.debug(
      `set canvas.style.width x canvas.style.height to ${width} x ${height}`
    );
  };

  // Default TdpClientEvent.TDP_CLIPBOARD_DATA handler.
  const clientOnClipboardData = async (clipboardData: ClipboardData) => {
    if (
      clipboardData.data &&
      (await shouldTryClipboardRW(clipboardSharingEnabled))
    ) {
      navigator.clipboard.writeText(clipboardData.data);
      let digest = await Sha256Digest(clipboardData.data, encoder.current);
      latestClipboardDigest.current = digest;
    }
  };

  // Default TdpClientEvent.TDP_ERROR and TdpClientEvent.CLIENT_ERROR handler
  const clientOnTdpError = (error: Error) => {
    setDirectorySharingState(prevState => ({
      ...prevState,
      isSharing: false,
    }));
    setClipboardSharingEnabled(false);
    setTdpConnection({
      status: 'failed',
      statusText: error.message || error.toString(),
    });
  };

  // Default TdpClientEvent.TDP_WARNING and TdpClientEvent.CLIENT_WARNING handler
  const clientOnTdpWarning = (warning: string) => {
    setWarnings(prevState => {
      return [
        ...prevState,
        {
          content: warning,
          severity: 'warn',
          id: crypto.randomUUID(),
        },
      ];
    });
  };

  const clientOnWsClose = () => {
    setWsConnection('closed');
  };

  const clientOnWsOpen = () => {
    setWsConnection('open');
  };

  /**
   * Returns the ButtonState corresponding to the given `keyArg`.
   *
   * @param e The `KeyboardEvent`
   * @param keyArg The key to check the state of. Valid values can be found [here](https://www.w3.org/TR/uievents-key/#keys-modifier)
   */
  const getModifierState = (e: KeyboardEvent, keyArg: string): ButtonState => {
    return e.getModifierState(keyArg) ? ButtonState.DOWN : ButtonState.UP;
  };

  const getSyncKeys = (e: KeyboardEvent): SyncKeys => {
    return {
      scrollLockState: getModifierState(e, 'ScrollLock'),
      numLockState: getModifierState(e, 'NumLock'),
      capsLockState: getModifierState(e, 'CapsLock'),
      kanaLockState: ButtonState.UP, // KanaLock is not supported, see https://www.w3.org/TR/uievents-key/#keys-modifier
    };
  };

  /**
   * Called before every keydown or keyup event.
   *
   * If syncBeforeNextKey is true, this function
   * synchronizes the keys to the remote machine.
   */
  const handleSyncBeforeNextKey = (cli: TdpClient, e: KeyboardEvent) => {
    if (syncBeforeNextKey.current === true) {
      cli.sendSyncKeys(getSyncKeys(e));
      syncBeforeNextKey.current = false;
    }
  };

  const isMac = getPlatform() === Platform.macOS;
  /**
   * Special handler for the CapsLock key.
   *
   * On MacOS Edge/Chrome/Safari, each physical CapsLock DOWN-UP registers
   * as either a single DOWN or single UP, with DOWN corresponding to
   * "CapsLock on" and UP to "CapsLock off". On MacOS Firefox, it always
   * registers as a DOWN.
   *
   * On Windows and Linux, all browsers treat CapsLock like a normal key.
   *
   * The remote Windows machine also treats CapsLock like a normal key, and
   * expects a DOWN-UP whenever it's pressed.
   */
  const handleCapsLock = (cli: TdpClient, state: ButtonState) => {
    if (isMac) {
      // On Mac, every UP or DOWN given to us by the browser corresponds
      // to a DOWN + UP on the remote machine.
      cli.sendKeyboardInput('CapsLock', ButtonState.DOWN);
      cli.sendKeyboardInput('CapsLock', ButtonState.UP);
    } else {
      // On Windows or Linux, we just pass the event through normally to the server.
      cli.sendKeyboardInput('CapsLock', state);
    }
  };

  /**
   * Handles a keyboard event.
   */
  const handleKeyboardEvent = (
    cli: TdpClient,
    e: KeyboardEvent,
    state: ButtonState
  ) => {
    if (e.code === 'CapsLock') {
      handleCapsLock(cli, state);
      return;
    }
    cli.sendKeyboardInput(e.code, state);
  };

  const canvasOnKeyDown = (cli: TdpClient, e: KeyboardEvent) => {
    e.preventDefault();
    handleSyncBeforeNextKey(cli, e);
    handleKeyboardEvent(cli, e, ButtonState.DOWN);

    // The key codes in the if clause below are those that have been empirically determined not
    // to count as transient activation events. According to the documentation, a keydown for
    // the Esc key and any "shortcut key reserved by the user agent" don't count as activation
    // events: https://developer.mozilla.org/en-US/docs/Web/Security/User_activation.
    if (
      e.code !== 'MetaRight' &&
      e.code !== 'MetaLeft' &&
      e.code !== 'AltRight' &&
      e.code !== 'AltLeft'
    ) {
      // Opportunistically sync local clipboard to remote while
      // transient user activation is in effect.
      // https://developer.mozilla.org/en-US/docs/Web/API/Clipboard/readText#security
      sendLocalClipboardToRemote(cli);
    }
  };

  const canvasOnKeyUp = (cli: TdpClient, e: KeyboardEvent) => {
    e.preventDefault();
    handleSyncBeforeNextKey(cli, e);
    handleKeyboardEvent(cli, e, ButtonState.UP);
  };

  const canvasOnFocusOut = () => {
    syncBeforeNextKey.current = true;
  };

  const canvasOnMouseMove = (
    cli: TdpClient,
    canvas: HTMLCanvasElement,
    e: MouseEvent
  ) => {
    const rect = canvas.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    cli.sendMouseMove(x, y);
  };

  const canvasOnMouseDown = (cli: TdpClient, e: MouseEvent) => {
    if (e.button === 0 || e.button === 1 || e.button === 2) {
      cli.sendMouseButton(e.button, ButtonState.DOWN);
    }

    // Opportunistically sync local clipboard to remote while
    // transient user activation is in effect.
    // https://developer.mozilla.org/en-US/docs/Web/API/Clipboard/readText#security
    sendLocalClipboardToRemote(cli);
  };

  const canvasOnMouseUp = (cli: TdpClient, e: MouseEvent) => {
    if (e.button === 0 || e.button === 1 || e.button === 2) {
      cli.sendMouseButton(e.button, ButtonState.UP);
    }
  };

  const canvasOnMouseWheelScroll = (cli: TdpClient, e: WheelEvent) => {
    e.preventDefault();
    // We only support pixel scroll events, not line or page events.
    // https://developer.mozilla.org/en-US/docs/Web/API/WheelEvent/deltaMode
    if (e.deltaMode === WheelEvent.DOM_DELTA_PIXEL) {
      if (e.deltaX) {
        cli.sendMouseWheelScroll(ScrollAxis.HORIZONTAL, -e.deltaX);
      }
      if (e.deltaY) {
        cli.sendMouseWheelScroll(ScrollAxis.VERTICAL, -e.deltaY);
      }
    }
  };

  // Block browser context menu so as not to obscure the context menu
  // on the remote machine.
  const canvasOnContextMenu = () => false;

  const sendLocalClipboardToRemote = async (cli: TdpClient) => {
    if (await shouldTryClipboardRW(clipboardSharingEnabled)) {
      navigator.clipboard.readText().then(text => {
        Sha256Digest(text, encoder.current).then(digest => {
          if (text && digest !== latestClipboardDigest.current) {
            cli.sendClipboardData({
              data: text,
            });
            latestClipboardDigest.current = digest;
          }
        });
      });
    }
  };

  return {
    tdpClient,
    clientScreenSpecToRequest: getDisplaySize(),
    clientOnPngFrame,
    clientOnBitmapFrame,
    clientOnClientScreenSpec,
    clientOnTdpError,
    clientOnClipboardData,
    clientOnWsClose,
    clientOnWsOpen,
    clientOnTdpWarning,
    canvasOnKeyDown,
    canvasOnKeyUp,
    canvasOnFocusOut,
    canvasOnMouseMove,
    canvasOnMouseDown,
    canvasOnMouseUp,
    canvasOnMouseWheelScroll,
    canvasOnContextMenu,
  };
}

// Calculates the size (in pixels) of the display.
// Since we want to maximize the display size for the user, this is simply
// the full width of the screen and the full height sans top bar.
function getDisplaySize() {
  return {
    width: window.innerWidth,
    height: window.innerHeight - TopBarHeight,
  };
}

type Props = {
  username: string;
  desktopName: string;
  clusterId: string;
  setTdpConnection: Dispatch<SetStateAction<Attempt>>;
  setWsConnection: Dispatch<SetStateAction<'open' | 'closed'>>;
  setClipboardSharingEnabled: Dispatch<SetStateAction<boolean>>;
  setDirectorySharingState: Dispatch<
    SetStateAction<{
      canShare: boolean;
      isSharing: boolean;
    }>
  >;
  clipboardSharingEnabled: boolean;
  setWarnings: Dispatch<SetStateAction<NotificationItem[]>>;
};

/**
 * To be called before any system clipboard read/write operation.
 *
 * @param clipboardSharingEnabled true if clipboard sharing is enabled by RBAC
 */
async function shouldTryClipboardRW(
  clipboardSharingEnabled: boolean
): Promise<boolean> {
  return (
    clipboardSharingEnabled &&
    document.hasFocus() && // if document doesn't have focus, clipboard r/w will throw an uncatchable error
    !(await isBrowserClipboardDenied()) // don't try r/w if either permission is denied
  );
}

/**
 * Returns true if either 'clipboard-read' or `clipboard-write' are 'denied',
 * false otherwise.
 *
 * This is used as a check before reading from or writing to the clipboard,
 * because we only want to do so when *both* read and write permissions are
 * granted (or if either one is 'prompt', which will cause the browser to
 * prompt the user to specify). This is because Chromium browsers default to
 * granting clipboard-write permissions, and only allow the user to toggle
 * clipboard-read. However the prompt makes it seem like the user is granting
 * or denying all clipboard permissions, which can lead to an awkward UX where
 * a user has explicitly denied clipboard permissions at the browser level,
 * but is still getting the remote clipboard contents synced to their local machine.
 *
 * By calling this function before any read or write transaction, we ensure we're
 * complying with the user's explicit intention towards our use of their clipboard.
 */
async function isBrowserClipboardDenied(): Promise<boolean> {
  const readPromise = navigator.permissions.query({
    name: 'clipboard-read' as PermissionName,
  });
  const writePromise = navigator.permissions.query({
    name: 'clipboard-write' as PermissionName,
  });
  const [readPerm, writePerm] = await Promise.all([readPromise, writePromise]);
  return readPerm.state === 'denied' || writePerm.state === 'denied';
}
