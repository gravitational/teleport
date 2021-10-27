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
import { useParams } from 'react-router';
import { TopBarHeight } from './TopBar';
import cfg, { UrlDesktopParams } from 'teleport/config';
import { getAccessToken, getHostName } from 'teleport/services/api';
import { ButtonState, ScrollAxis } from 'teleport/lib/tdp/codec';
import useAttempt from 'shared/hooks/useAttemptNext';

export default function useTdpClientCanvas() {
  const { clusterId, username, desktopId } = useParams<UrlDesktopParams>();
  // status === '' means disconnected
  const {
    attempt: connectionAttempt,
    setAttempt: setConnectionAttempt,
  } = useAttempt('processing');

  // Build a client based on url parameters.
  const tdpClient = useMemo(() => {
    const addr = cfg.api.desktopWsAddr
      .replace(':fqdm', getHostName())
      .replace(':clusterId', clusterId)
      .replace(':desktopId', desktopId)
      .replace(':token', getAccessToken());

    return new TdpClient(addr, username);
  }, [clusterId, username, desktopId]);

  const syncCanvasSizeToClientSize = (canvas: HTMLCanvasElement) => {
    // Calculate the size of the canvas to be displayed.
    // Setting flex to "1" ensures the canvas will fill out the area available to it,
    // which we calculate based on the window dimensions and TopBarHeight below.
    const width = window.innerWidth;
    const height = window.innerHeight - TopBarHeight;

    // If it's resolution does not match change it
    if (canvas.width !== width || canvas.height !== height) {
      canvas.width = width;
      canvas.height = height;
    }
  };

  const onInit = (cli: TdpClient, canvas: HTMLCanvasElement) => {
    setConnectionAttempt({ status: 'processing' });
    syncCanvasSizeToClientSize(canvas);
    cli.connect(canvas.width, canvas.height);
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
