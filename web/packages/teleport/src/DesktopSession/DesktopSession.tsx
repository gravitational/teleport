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

import React, {
  useEffect,
  useRef,
  Dispatch,
  SetStateAction,
  CSSProperties,
} from 'react';
import useDesktopSession, {
  State,
  TdpClientConnectionState,
} from './useDesktopSession';
import TopBar, { TopBarHeight } from './TopBar';
import { Indicator, Box, Alert, Text, Flex } from 'design';
import useTeleport from 'teleport/useTeleport';
import TdpClient from 'teleport/lib/tdp/client';

export default function Container() {
  const ctx = useTeleport();
  const state = useDesktopSession(ctx);
  return <DesktopSession {...state} />;
}

export function DesktopSession(props: State) {
  const {
    hostname,
    tdpClient,
    attempt,
    clipboard,
    recording,
    username,
    connection,
    setConnection,
  } = props;

  const displayStatusMessage = () => {
    if (attempt.status === 'failed' || connection.status === 'error') {
      return (
        <Alert
          style={{
            alignSelf: 'center',
          }}
          width={'450px'}
          my={2}
          children={
            attempt.status === 'failed'
              ? attempt.statusText
              : connection.statusText
          }
        />
      );
    } else if (
      attempt.status === 'processing' ||
      connection.status === 'connecting'
    ) {
      return (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      );
    } else if (connection.status === 'disconnected') {
      return (
        <Box textAlign="center" m={10}>
          <Text>Session successfully disconnected</Text>
        </Box>
      );
    } else if (
      attempt.status === 'success' &&
      connection.status === 'connected'
    ) {
      return null;
    } else {
      <Alert
        style={{
          alignSelf: 'center',
        }}
        width={'450px'}
        my={2}
        children={'unexpected state'}
      />;
    }
  };

  return (
    <Flex flexDirection="column">
      <TopBar
        onDisconnect={() => {
          tdpClient.disconnect();
        }}
        userHost={`${username}@${hostname}`}
        clipboard={clipboard}
        recording={recording}
      />
      {displayStatusMessage()}
      <TDPClientCanvas
        style={{
          display:
            attempt.status === 'success' && connection.status === 'connected'
              ? 'flex'
              : 'none',
          flex: 1,
        }}
        tdpClient={tdpClient}
        setConnection={setConnection}
        syncCanvasSizeToClientSize={(canvas: HTMLCanvasElement) => {
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
        }}
      />
    </Flex>
  );
}

function TDPClientCanvas(props: {
  tdpClient: TdpClient;
  // syncCanvasSizeToClientSize is a function for sync-ing the canvas's internal size (canvas.width/height)
  // with the size of the canvas displayed on screen. Called when TDPClientCanvas is first rendered to give
  // tdp server the initial screen size target, and called on subsequent changes in client window size (TODO).
  syncCanvasSizeToClientSize: (canvas: HTMLCanvasElement) => void;
  setConnection: Dispatch<SetStateAction<TdpClientConnectionState>>;
  style?: CSSProperties;
}) {
  const { tdpClient, setConnection, style, syncCanvasSizeToClientSize } = props;
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    tdpClient.on('render', ({ bitmap, left, top }) => {
      const ctx = canvasRef.current.getContext('2d');
      ctx.drawImage(bitmap, left, top);
    });

    tdpClient.on('disconnect', () => {
      setConnection({
        status: 'disconnected',
      });
    });

    tdpClient.on('error', (err: Error) => {
      setConnection({ status: 'error', statusText: err.message });
    });

    tdpClient.on('init', () => {
      setConnection({ status: 'connecting' });
      syncCanvasSizeToClientSize(canvasRef.current);
      tdpClient.connect(canvasRef.current.width, canvasRef.current.height);
    });

    tdpClient.on('connect', () => {
      setConnection({ status: 'connected' });
    });

    tdpClient.init();

    return () => {
      tdpClient.nuke();
    };
  }, [tdpClient]);

  return <canvas style={{ ...style }} ref={canvasRef} />;
}
