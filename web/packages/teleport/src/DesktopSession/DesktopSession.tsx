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

import React from 'react';
import styled from 'styled-components';
import useDesktopSession, { State } from './useDesktopSession';
import TopBar from './TopBar';
import { Indicator, Box, Alert } from 'design';
import useTeleport from 'teleport/useTeleport';

export default function Container() {
  const ctx = useTeleport();
  const state = useDesktopSession(ctx);
  return <DesktopSession {...state} />;
}

export function DesktopSession(props: State) {
  const { userHost, setWsAttempt, tdpClient, attempt } = props;
  const canvasRef = React.useRef<HTMLCanvasElement>(null);

  // Waits for the state hook to initialize the TdpClient.
  // Once the client is initialized, sets wsAttempt to 'success'.
  React.useEffect(() => {
    setWsAttempt({ status: 'processing' });

    tdpClient.on('open', () => {
      // set wsAttempt to success, triggering
      // useDesktopSession.useEffect(..., [wsAttempt, fetchDesktopAttempt])
      // to update the meta attempt state
      setWsAttempt({ status: 'success' });
    });

    // If the websocket is closed remove all listeners that depend on it.
    tdpClient.on('close', () => {
      // TODO: this shouldn't necessarily be a failure, i.e. if the session times out or
      // if the user selects "disconnect" from the menu
      setWsAttempt({
        status: 'failed',
        statusText: 'connection to remote server was closed',
      });
      tdpClient.removeAllListeners();
    });

    tdpClient.on('error', (err: Error) => {
      setWsAttempt({
        status: 'failed',
        statusText: err.message,
      });
    });

    tdpClient.on('render', ({ bitmap, left, top }) => {
      const ctx = canvasRef.current.getContext('2d');
      ctx.drawImage(bitmap, left, top);
    });

    // Connect to the websocket, triggering tdpClient.on('open') above.
    tdpClient.connect();

    // If client parameters change or component will unmount, close the websocket.
    return () => {
      tdpClient.disconnect();
    };
  }, [tdpClient]);

  React.useEffect(() => {
    // When attempt is set to 'success' after both the websocket connection and api call have succeeded,
    // the canvas component gets rendered at which point we can send its width and height to the tdpClient
    // as part of the TDP initial handshake.
    if (attempt.status === 'success') {
      syncCanvasSizeToClientSize(canvasRef.current);
      tdpClient.sendUsername();
      tdpClient.resize(canvasRef.current.width, canvasRef.current.height);
    }
  }, [attempt]);

  // Canvas has two size attributes: the dimension of the pixels in the canvas (canvas.width)
  // and the display size of the html element (canvas.clientWidth). syncCanvasSizeToClientSize
  // ensures the two remain equal.
  function syncCanvasSizeToClientSize(canvas: HTMLCanvasElement) {
    // look up the size the canvas is being displayed
    const width = canvas.clientWidth;
    const height = canvas.clientHeight;

    // If it's resolution does not match change it
    if (canvas.width !== width || canvas.height !== height) {
      canvas.width = width;
      canvas.height = height;
    }
  }

  return (
    <StyledDesktopSession>
      <TopBar
        userHost={userHost}
        // clipboard and recording settings will eventuall come from backend, hardcoded for now
        clipboard={false}
        recording={false}
        attempt={attempt}
      />
      {attempt.status === 'failed' && (
        <Alert mx={10} my={2} children={attempt.statusText} />
      )}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}

      {attempt.status === 'success' && (
        <>
          <canvas ref={canvasRef} />
        </>
      )}
    </StyledDesktopSession>
  );
}

// Ensures the UI fills the entire available screen space.
const StyledDesktopSession = styled.div`
  bottom: 0;
  left: 0;
  position: absolute;
  right: 0;
  top: 0;
  display: flex;
  flex-direction: column;
`;
