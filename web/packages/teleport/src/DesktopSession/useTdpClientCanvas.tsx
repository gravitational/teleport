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

import { useState, useMemo } from 'react';
import TdpClient, { RenderData } from 'teleport/lib/tdp/client';
import { useParams } from 'react-router';
import { TopBarHeight } from './TopBar';
import cfg, { UrlDesktopParams } from 'teleport/config';
import { getAccessToken, getHostName } from 'teleport/services/api';

export default function useTdpClientCanvas() {
  const { clusterId, username, desktopId } = useParams<UrlDesktopParams>();
  const [connection, setConnection] = useState<TdpClientConnectionState>({
    status: 'connecting',
  });

  // Build a client based on url parameters.
  const tdpClient = useMemo(() => {
    const addr = cfg.api.desktopWsAddr
      .replace(':fqdm', getHostName())
      .replace(':clusterId', clusterId)
      .replace(':desktopId', desktopId)
      .replace(':token', getAccessToken());

    return new TdpClient(addr, username);
  }, [clusterId, username, desktopId]);

  const onInit = (canvas: HTMLCanvasElement, cli: TdpClient) => {
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

    setConnection({ status: 'connecting' });
    syncCanvasSizeToClientSize(canvas);
    cli.connect(canvas.width, canvas.height);
  };

  const onConnect = () => {
    setConnection({ status: 'connected' });
  };

  const onRender = (canvas: HTMLCanvasElement, data: RenderData) => {
    const ctx = canvas.getContext('2d');
    ctx.drawImage(data.bitmap, data.left, data.top);
  };

  const onDisconnect = () => {
    setConnection({
      status: 'disconnected',
    });
  };

  const onError = (err: Error) => {
    setConnection({ status: 'error', statusText: err.message });
  };

  return {
    tdpClient,
    connection,
    username,
    onInit,
    onConnect,
    onRender,
    onDisconnect,
    onError,
  };
}

export type TdpClientConnectionState = {
  status: 'connecting' | 'connected' | 'disconnected' | 'error';
  statusText?: string;
};
