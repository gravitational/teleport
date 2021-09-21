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
import useDesktopSession, { State } from './useDesktopSession';
import TopBar from './TopBar';
import { Indicator, Box, Alert, Text, Flex } from 'design';
import useTeleport from 'teleport/useTeleport';
import TdpClientCanvas from './TdpClientCanvas';

export default function Container() {
  const ctx = useTeleport();
  const state = useDesktopSession(ctx);
  return <DesktopSession {...state} />;
}

export function DesktopSession(props: State) {
  const {
    hostname,
    attempt,
    clipboard,
    recording,
    tdpClient,
    connection,
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
    onResize,
  } = props;

  const errorAlert = (
    <Alert
      style={{
        alignSelf: 'center',
      }}
      width={'450px'}
      my={2}
      children={
        attempt.status === 'failed'
          ? attempt.statusText
          : connection.status === 'error'
          ? connection.statusText
          : 'unexpected state'
      }
    />
  );

  // Calculates an optional status message for the UI based on the combined state
  // of attempt and connection.
  const displayStatusMessage = () => {
    if (attempt.status === 'failed' || connection.status === 'error') {
      return errorAlert;
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
      return errorAlert;
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

      <TdpClientCanvas
        style={{
          display:
            attempt.status === 'success' && connection.status === 'connected'
              ? 'flex'
              : 'none',
          flex: 1,
        }}
        tdpClient={tdpClient}
        connection={connection}
        username={username}
        onInit={onInit}
        onConnect={onConnect}
        onRender={onRender}
        onDisconnect={onDisconnect}
        onError={onError}
        onKeyDown={onKeyDown}
        onKeyUp={onKeyUp}
        onMouseMove={onMouseMove}
        onMouseDown={onMouseDown}
        onMouseUp={onMouseUp}
        onResize={onResize}
      />
    </Flex>
  );
}
