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
import { Indicator, Box, Alert, Text } from 'design';
import useTeleport from 'teleport/useTeleport';

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
    setAttempt,
    clipboard,
    recording,
    username,
  } = props;
  const canvasRef = React.useRef<HTMLCanvasElement>(null);

  React.useEffect(() => {
    // When attempt is set to 'success' that means both the websocket connection and initial api call(s) have succeeded.
    // Now the canvas gets rendered, at which point we can set up all the tdpClient's listeners and initialize the tdp
    // connection using the canvas' initial size
    if (attempt.status === 'success') {
      const canvas = canvasRef.current;

      tdpClient.on('render', ({ bitmap, left, top }) => {
        const ctx = canvasRef.current.getContext('2d');
        ctx.drawImage(bitmap, left, top);
      });

      tdpClient.on('disconnect', () => {
        setAttempt({ status: 'disconnected' });
      });

      tdpClient.on('error', (err: Error) => {
        setAttempt({
          status: 'failed',
          statusText: err.message ? err.message : 'unknown error',
        });
      });

      syncCanvasSizeToClientSize(canvasRef.current);
      tdpClient.init(username, canvas.width, canvas.height);
    }

    // If client parameters change or component will unmount, cleanup tdpClient.
    return () => {
      // If the previous attempt was a 'success' that means tdpClient was connected.
      // Since the connection is no longer a 'success', clean up the connection.
      if (attempt.status === 'success') {
        // Remove all listeners first, so that tdpClient.disconnect() does not trigger the 'disconnect' handler above.
        tdpClient.removeAllListeners();
        tdpClient.disconnect();
      }
    };
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
        onDisconnect={() => {
          tdpClient.disconnect();
        }}
        userHost={`${username}@${hostname}`}
        clipboard={clipboard}
        recording={recording}
        attempt={attempt}
      />
      {attempt.status === 'failed' && (
        <Alert
          style={{
            alignSelf: 'center',
          }}
          width={'450px'}
          my={2}
          children={attempt.statusText}
        />
      )}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}

      {attempt.status === 'disconnected' && (
        <Box textAlign="center" m={10}>
          <Text>Remote desktop successfully disconnected.</Text>
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
