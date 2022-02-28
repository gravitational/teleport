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

import { useState, useEffect, useRef, Dispatch, SetStateAction } from 'react';
import { TdpClient, ButtonState, ScrollAxis } from 'teleport/lib/tdp';
import { ClipboardData, PngFrame } from 'teleport/lib/tdp/codec';
import { TopBarHeight } from './TopBar';
import cfg from 'teleport/config';
import { getAccessToken, getHostName } from 'teleport/services/api';
import { Attempt } from 'shared/hooks/useAttemptNext';

export default function useTdpClientCanvas(props: Props) {
  const {
    username,
    desktopName,
    clusterId,
    setTdpConnection,
    setWsConnection,
    enableClipboardSharing,
  } = props;
  const [tdpClient, setTdpClient] = useState<TdpClient | null>(null);
  const initialTdpConnectionSucceeded = useRef(false);

  useEffect(() => {
    const { width, height } = getDisplaySize();

    const addr = cfg.api.desktopWsAddr
      .replace(':fqdn', getHostName())
      .replace(':clusterId', clusterId)
      .replace(':desktopName', desktopName)
      .replace(':token', getAccessToken())
      .replace(':username', username)
      .replace(':width', width.toString())
      .replace(':height', height.toString());

    setTdpClient(new TdpClient(addr));
  }, [clusterId, username, desktopName]);

  const syncCanvasSizeToDisplaySize = (canvas: HTMLCanvasElement) => {
    const { width, height } = getDisplaySize();

    canvas.width = width;
    canvas.height = height;
  };

  // Default TdpClientEvent.TDP_PNG_FRAME handler (buffered)
  const onPngFrame = (ctx: CanvasRenderingContext2D, pngFrame: PngFrame) => {
    // The first image fragment we see signals a successful tdp connection.
    if (!initialTdpConnectionSucceeded.current) {
      syncCanvasSizeToDisplaySize(ctx.canvas);
      setTdpConnection({ status: 'success' });
      initialTdpConnectionSucceeded.current = true;
    }
    ctx.drawImage(pngFrame.data, pngFrame.left, pngFrame.top);
  };

  // Default TdpClientEvent.TDP_CLIPBOARD_DATA handler.
  const onClipboardData = (clipboardData: ClipboardData) => {
    if (enableClipboardSharing && document.hasFocus()) {
      navigator.clipboard.writeText(clipboardData.data);
    }
  };

  // Default TdpClientEvent.TDP_ERROR handler
  const onTdpError = (err: Error) => {
    setTdpConnection({ status: 'failed', statusText: err.message });
  };

  const onWsClose = () => {
    setWsConnection('closed');
  };

  const onWsOpen = () => {
    setWsConnection('open');
  };

  const onKeyDown = (cli: TdpClient, e: KeyboardEvent) => {
    e.preventDefault();
    cli.sendKeyboardInput(e.code, ButtonState.DOWN);
  };

  const onKeyUp = (cli: TdpClient, e: KeyboardEvent) => {
    e.preventDefault();
    cli.sendKeyboardInput(e.code, ButtonState.UP);
  };

  const onMouseMove = (
    cli: TdpClient,
    canvas: HTMLCanvasElement,
    e: MouseEvent
  ) => {
    const rect = canvas.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const y = e.clientY - rect.top;
    cli.sendMouseMove(x, y);
  };

  const onMouseDown = (cli: TdpClient, e: MouseEvent) => {
    if (e.button === 0 || e.button === 1 || e.button === 2) {
      cli.sendMouseButton(e.button, ButtonState.DOWN);
    }
  };

  const onMouseUp = (cli: TdpClient, e: MouseEvent) => {
    if (e.button === 0 || e.button === 1 || e.button === 2) {
      cli.sendMouseButton(e.button, ButtonState.UP);
    }
  };

  const onMouseWheelScroll = (cli: TdpClient, e: WheelEvent) => {
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
  const onContextMenu = () => false;

  const sendLocalClipboardToRemote = (cli: TdpClient) => {
    // We must check that the DOM is focused or navigator.clipboard.readText throws an error.
    if (enableClipboardSharing && document.hasFocus()) {
      navigator.clipboard.readText().then(text => {
        cli.sendClipboardData({
          data: text,
        });
      });
    }
  };

  // Syncs the browser-side's clipboard. See the note about mouseenter in the relevant RFD for why this makes sense:
  // https://github.com/gravitational/teleport/blob/master/rfd/0049-desktop-clipboard.md#local-copy-remote-paste
  const onMouseEnter = (cli: TdpClient, e: MouseEvent) => {
    e.preventDefault();
    sendLocalClipboardToRemote(cli);
  };

  // onMouseEnter does not fire in certain situations, so ensure we cover all of our bases by adding a window level
  // onfocus handler. See https://github.com/gravitational/webapps/issues/626 for further details.
  const windowOnFocus = (cli: TdpClient, e: FocusEvent) => {
    e.preventDefault();
    sendLocalClipboardToRemote(cli);
  };

  return {
    tdpClient,
    onPngFrame,
    onTdpError,
    onClipboardData,
    onWsClose,
    onWsOpen,
    onKeyDown,
    onKeyUp,
    onMouseMove,
    onMouseDown,
    onMouseUp,
    onMouseWheelScroll,
    onContextMenu,
    onMouseEnter,
    windowOnFocus,
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
  enableClipboardSharing: boolean;
};
