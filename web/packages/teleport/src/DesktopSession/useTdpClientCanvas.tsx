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

import { useState, useEffect, useRef } from 'react';
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
import { getHostName } from 'teleport/services/api';
import cfg from 'teleport/config';
import { Sha256Digest } from 'teleport/lib/util';

import { TopBarHeight } from './TopBar';
import {
  ClipboardSharingState,
  DirectorySharingState,
  Setter,
  clipboardSharingPossible,
  defaultClipboardSharingState,
  defaultDirectorySharingState,
  isSharingClipboard,
} from './useDesktopSession';

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
    clipboardSharingState,
    setClipboardSharingState,
    setDirectorySharingState,
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

  const keyboardHandler = useRef(new KeyboardHandler());

  useEffect(() => {
    const addr = cfg.api.desktopWsAddr
      .replace(':fqdn', getHostName())
      .replace(':clusterId', clusterId)
      .replace(':desktopName', desktopName)
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
      (await sysClipboardGuard(clipboardSharingState, 'write'))
    ) {
      navigator.clipboard.writeText(clipboardData.data);
      let digest = await Sha256Digest(clipboardData.data, encoder.current);
      latestClipboardDigest.current = digest;
    }
  };

  // Default TdpClientEvent.TDP_ERROR and TdpClientEvent.CLIENT_ERROR handler
  const clientOnTdpError = (error: Error) => {
    setDirectorySharingState(defaultDirectorySharingState);
    setClipboardSharingState(defaultClipboardSharingState);
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

  const clientOnTdpInfo = (info: string) => {
    setDirectorySharingState(defaultDirectorySharingState);
    setClipboardSharingState(defaultClipboardSharingState);
    setTdpConnection({
      status: '', // gracefully disconnecting
      statusText: info,
    });
  };

  const clientOnWsClose = (statusText: string) => {
    setWsConnection({ status: 'closed', statusText });
  };

  const clientOnWsOpen = () => {
    setWsConnection({ status: 'open' });
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

  const canvasOnKeyDown = (cli: TdpClient, e: KeyboardEvent) => {
    e.preventDefault();
    handleSyncBeforeNextKey(cli, e);
    keyboardHandler.current.handleKeyboardEvent(cli, e, ButtonState.DOWN);

    // The key codes in the if clause below are those that have been empirically determined not
    // to count as transient activation events. According to the documentation, a keydown for
    // the Esc key and any "shortcut key reserved by the user agent" don't count as activation
    // events: https://developer.mozilla.org/en-US/docs/Web/Security/User_activation.
    if (e.key !== 'Meta' && e.key !== 'Alt' && e.key !== 'Escape') {
      // Opportunistically sync local clipboard to remote while
      // transient user activation is in effect.
      // https://developer.mozilla.org/en-US/docs/Web/API/Clipboard/readText#security
      sendLocalClipboardToRemote(cli);
    }
  };

  const canvasOnKeyUp = (cli: TdpClient, e: KeyboardEvent) => {
    e.preventDefault();
    handleSyncBeforeNextKey(cli, e);
    keyboardHandler.current.handleKeyboardEvent(cli, e, ButtonState.UP);
  };

  const canvasOnFocusOut = () => {
    keyboardHandler.current.handleOnFocusOut();
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
    if (await sysClipboardGuard(clipboardSharingState, 'read')) {
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
    clientOnTdpInfo,
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
  setTdpConnection: Setter<Attempt>;
  setWsConnection: Setter<{ status: 'open' | 'closed'; statusText?: string }>;
  clipboardSharingState: ClipboardSharingState;
  setClipboardSharingState: Setter<ClipboardSharingState>;
  setDirectorySharingState: Setter<DirectorySharingState>;
  setWarnings: Setter<NotificationItem[]>;
};

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

class KeyboardHandler {
  withheldDown: { [code: string]: boolean | undefined } = {};
  delayedUp: { [code: string]: NodeJS.Timeout | undefined } = {};
  private isMac: boolean;

  constructor() {
    this.isMac = getPlatform() === Platform.macOS;
  }

  public handleKeyboardEvent(
    cli: TdpClient,
    e: KeyboardEvent,
    state: ButtonState
  ) {
    if (this.handleCapsLock(cli, e, state)) {
      return;
    }

    if (this.handleWithholdingAndDelay(cli, e, state)) {
      return;
    }

    this.handleUnwithholding(cli, e, state);

    cli.sendKeyboardInput(e.code, state);
  }

  /**
   * Called when the canvas loses focus.
   *
   * This clears the withheld and delayed keys, so that they are not sent
   * to the server when the canvas is out of focus.
   */
  public handleOnFocusOut() {
    this.clearWithheldDown();
    this.clearDelayedUp();
  }

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
   *
   * Returns true if the event was handled, false otherwise.
   */
  private handleCapsLock(
    cli: TdpClient,
    e: KeyboardEvent,
    state: ButtonState
  ): boolean {
    if (e.code === 'CapsLock') {
      if (this.isMac) {
        // On Mac, every UP or DOWN given to us by the browser corresponds
        // to a DOWN + UP on the remote machine.
        cli.sendKeyboardInput('CapsLock', ButtonState.DOWN);
        cli.sendKeyboardInput('CapsLock', ButtonState.UP);
      } else {
        // On Windows or Linux, we just pass the event through normally to the server.
        cli.sendKeyboardInput('CapsLock', state);
      }

      return true;
    }

    return false;
  }

  /**
   * Called before every keydown or keyup event. This witholds the
   * keys for which we want to see what the next event is before
   * sending them on to the server.
   *
   * Returns true if the event was handled, false otherwise.
   */
  private handleWithholdingAndDelay(
    cli: TdpClient,
    e: KeyboardEvent,
    state: ButtonState
  ): boolean {
    if (this.isWitholdableKey(e) && state === ButtonState.DOWN) {
      // Unlikely, but theoretically possible. In order to ensure correctness,
      // we clear any delayed up event for this key and handle it immediately.
      const timeout = this.delayedUp[e.code];
      if (timeout) {
        clearTimeout(timeout);
        this.handleUnwithholding(cli, e, state);
        cli.sendKeyboardInput(e.code, ButtonState.UP);
        this.delayedUp[e.code] = undefined;
      }

      // Then we set the key down to be withheld until the next keydown or keyup,
      // or to never be sent if this cache is cleared onfocusout.
      this.withheldDown[e.code] = true;

      return true;
    } else if (this.isWitholdableKey(e) && state === ButtonState.UP) {
      const timeout = setTimeout(() => {
        this.handleUnwithholding(cli, e, state);
        cli.sendKeyboardInput(e.code, ButtonState.UP);
        this.delayedUp[e.code] = undefined;
      }, 5 /* ms */);

      // And add the timeout to the cache.
      this.delayedUp[e.code] = timeout;

      return true;
    }

    return false;
  }

  /**
   * Called after every keydown or keyup event. This sends any currently
   * withheld keys to the server.
   */
  private handleUnwithholding(
    cli: TdpClient,
    e: KeyboardEvent,
    state: ButtonState
  ) {
    if (this.shouldBeWithheld(e, state)) {
      console.error('Unwithholding a key event that should have been withheld');
      return;
    }

    for (const code in this.withheldDown) {
      cli.sendKeyboardInput(code, ButtonState.DOWN);
      this.withheldDown[code] = undefined;
    }

    this.clearWithheldDown();
  }

  private isWitholdableKey(e: KeyboardEvent): boolean {
    return e.key === 'Meta' || e.key === 'Alt';
  }

  private shouldBeWithheld(e: KeyboardEvent, state: ButtonState): boolean {
    return this.isWitholdableKey(e) && state === ButtonState.DOWN;
  }

  private clearWithheldDown() {
    this.withheldDown = {};
  }

  private clearDelayedUp() {
    this.delayedUp = {};
  }
}
