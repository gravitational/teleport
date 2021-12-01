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

import { useMemo } from 'react';
import TdpClient, { ImageData } from 'teleport/lib/tdp/client';
import { TopBarHeight } from './TopBar';
import cfg from 'teleport/config';
import { getAccessToken, getHostName } from 'teleport/services/api';
import { ButtonState, ScrollAxis } from 'teleport/lib/tdp/codec';
import useAttempt from 'shared/hooks/useAttemptNext';

export default function useTdpClientCanvas(props: Props) {
  const { username, desktopName, clusterId } = props;
  // status === '' means disconnected
  const {
    attempt: connectionAttempt,
    setAttempt: setConnectionAttempt,
  } = useAttempt('processing');

  // Build a client based on url parameters.
  const tdpClient = useMemo(() => {
    const { width, height } = getDisplaySize();

    const addr = cfg.api.desktopWsAddr
      .replace(':fqdm', getHostName())
      .replace(':clusterId', clusterId)
      .replace(':desktopName', desktopName)
      .replace(':token', getAccessToken())
      .replace(':username', username)
      .replace(':width', width.toString())
      .replace(':height', height.toString());

    return new TdpClient(addr, username);
  }, [clusterId, username, desktopName]);

  const syncCanvasSizeToDisplaySize = (canvas: HTMLCanvasElement) => {
    const { width, height } = getDisplaySize();

    canvas.width = width;
    canvas.height = height;
  };

  const onInit = (canvas: HTMLCanvasElement) => {
    setConnectionAttempt({ status: 'processing' });
    syncCanvasSizeToDisplaySize(canvas);
  };

  const onConnect = () => {
    setConnectionAttempt({ status: 'success' });
  };

  const onRender = (ctx: CanvasRenderingContext2D, data: ImageData) => {
    ctx.drawImage(data.image, data.left, data.top);
  };

  const onDisconnect = () => {
    setConnectionAttempt({
      status: '',
    });
  };

  const onError = (err: Error) => {
    setConnectionAttempt({ status: 'failed', statusText: err.message });
  };

  const onKeyDown = (cli: TdpClient, e: KeyboardEvent) => {
    cli.sendKeyboardInput(e.code, ButtonState.DOWN);
  };

  const onKeyUp = (cli: TdpClient, e: KeyboardEvent) => {
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

  return {
    tdpClient,
    connectionAttempt,
    username,
    onInit,
    onConnect,
    onRender,
    onDisconnect,
    onError,
    onKeyDown,
    onKeyUp,
    onMouseMove,
    onMouseDown,
    onMouseUp,
    onMouseWheelScroll,
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
};
