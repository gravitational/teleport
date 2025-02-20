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

import React, { useEffect, useRef, useState } from 'react';

import { NotificationItem } from 'shared/components/Notification';
import { Attempt } from 'shared/hooks/useAttemptNext';
import { debounce } from 'shared/utils/highbar';

import cfg from 'teleport/config';
import { ButtonState, ScrollAxis, TdpClient } from 'teleport/lib/tdp';
import type { BitmapFrame } from 'teleport/lib/tdp/client';
import {
  ClientScreenSpec,
  ClipboardData,
  PngFrame,
} from 'teleport/lib/tdp/codec';
import { Sha256Digest } from 'teleport/lib/util';
import { getHostName } from 'teleport/services/api';

import { KeyboardHandler } from './KeyboardHandler';
import { TopBarHeight } from './TopBar';
import {
  clipboardSharingPossible,
  ClipboardSharingState,
  defaultClipboardSharingState,
  defaultDirectorySharingState,
  DirectorySharingState,
  isSharingClipboard,
  Setter,
} from './useDesktopSession';

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
    setAlerts,
  } = props;
  const [tdpClient, setTdpClient] = useState<TdpClient | null>(null);
  const initialTdpConnectionSucceeded = useRef(false);
  const encoder = useRef(new TextEncoder());
  const latestClipboardDigest = useRef('');
  const keyboardHandler = useRef(new KeyboardHandler());

  useEffect(() => {
    keyboardHandler.current = new KeyboardHandler();
    // On unmount, clear all the timeouts on the keyboardHandler.
    return () => {
      // eslint-disable-next-line react-hooks/exhaustive-deps
      keyboardHandler.current.dispose();
    };
  }, []);

  useEffect(() => {
    const addr = cfg.api.desktopWsAddr
      .replace(':fqdn', getHostName())
      .replace(':clusterId', clusterId)
      .replace(':desktopName', desktopName)
      .replace(':username', username);

    setTdpClient(new TdpClient(addr));
  }, [clusterId, username, desktopName]);

  /**
   * Synchronize the canvas resolution and display size with the
   * given ClientScreenSpec.
   */
  const syncCanvas = (canvas: HTMLCanvasElement, spec: ClientScreenSpec) => {
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

  // Default TdpClientEvent.TDP_PNG_FRAME handler (buffered)
  const clientOnPngFrame = (
    ctx: CanvasRenderingContext2D,
    pngFrame: PngFrame
  ) => {
    // The first image fragment we see signals a successful TDP connection.
    if (!initialTdpConnectionSucceeded.current) {
      syncCanvas(ctx.canvas, getDisplaySize());
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
    syncCanvas(canvas, spec);
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
    setTdpConnection(prevState => {
      // Sometimes when a connection closes due to an error, we get a cascade of
      // errors. Here we update the status only if it's not already 'failed', so
      // that the first error message (which is usually the most informative) is
      // displayed to the user.
      if (prevState.status !== 'failed') {
        return {
          status: 'failed',
          statusText: error.message || error.toString(),
        };
      }
      return prevState;
    });
  };

  // Default TdpClientEvent.TDP_WARNING and TdpClientEvent.CLIENT_WARNING handler
  const clientOnTdpWarning = (warning: string) => {
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
  };

  // TODO(zmb3): this is not what an info-level alert should do.
  // rename it to something like onGracefulDisconnect
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

  const canvasOnKeyDown = (e: React.KeyboardEvent) => {
    keyboardHandler.current.handleKeyboardEvent({
      cli: tdpClient,
      e: e.nativeEvent,
      state: ButtonState.DOWN,
    });

    // The key codes in the if clause below are those that have been empirically determined not
    // to count as transient activation events. According to the documentation, a keydown for
    // the Esc key and any "shortcut key reserved by the user agent" don't count as activation
    // events: https://developer.mozilla.org/en-US/docs/Web/Security/User_activation.
    if (e.key !== 'Meta' && e.key !== 'Alt' && e.key !== 'Escape') {
      // Opportunistically sync local clipboard to remote while
      // transient user activation is in effect.
      // https://developer.mozilla.org/en-US/docs/Web/API/Clipboard/readText#security
      sendLocalClipboardToRemote(tdpClient);
    }
  };

  const canvasOnKeyUp = (e: React.KeyboardEvent) => {
    keyboardHandler.current.handleKeyboardEvent({
      cli: tdpClient,
      e: e.nativeEvent,
      state: ButtonState.UP,
    });
  };

  const canvasOnFocusOut = () => {
    keyboardHandler.current.onFocusOut();
  };

  const canvasOnMouseMove = (e: React.MouseEvent) => {
    const rect = e.currentTarget.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    tdpClient.sendMouseMove(x, y);
  };

  const canvasOnMouseDown = (e: React.MouseEvent) => {
    if (e.button === 0 || e.button === 1 || e.button === 2) {
      tdpClient.sendMouseButton(e.button, ButtonState.DOWN);
    }

    // Opportunistically sync local clipboard to remote while
    // transient user activation is in effect.
    // https://developer.mozilla.org/en-US/docs/Web/API/Clipboard/readText#security
    sendLocalClipboardToRemote(tdpClient);
  };

  const canvasOnMouseUp = (e: React.MouseEvent) => {
    if (e.button === 0 || e.button === 1 || e.button === 2) {
      tdpClient.sendMouseButton(e.button, ButtonState.UP);
    }
  };

  const canvasOnMouseWheelScroll = (e: WheelEvent) => {
    e.preventDefault();
    // We only support pixel scroll events, not line or page events.
    // https://developer.mozilla.org/en-US/docs/Web/API/WheelEvent/deltaMode
    if (e.deltaMode === WheelEvent.DOM_DELTA_PIXEL) {
      if (e.deltaX) {
        tdpClient.sendMouseWheelScroll(ScrollAxis.HORIZONTAL, -e.deltaX);
      }
      if (e.deltaY) {
        tdpClient.sendMouseWheelScroll(ScrollAxis.VERTICAL, -e.deltaY);
      }
    }
  };

  // Block browser context menu so as not to obscure the context menu
  // on the remote machine.
  const canvasOnContextMenu = (e: React.MouseEvent) => {
    e.preventDefault();
  };

  const windowOnResize = debounce(
    (cli: TdpClient) => {
      const spec = getDisplaySize();
      cli.resize(spec);
    },
    250,
    { trailing: true }
  );

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
    windowOnResize,
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
  setAlerts: Setter<NotificationItem[]>;
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
