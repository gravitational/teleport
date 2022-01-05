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
import TdpClientCanvas from 'teleport/components/TdpClientCanvas';

export default function Container() {
  const ctx = useTeleport();
  const state = useDesktopSession(ctx);
  return <DesktopSession {...state} />;
}

export function DesktopSession(props: State) {
  const {
    hostname,
    username,
    clipboard,
    recording,
    tdpClient,
    fetchAttempt,
    tdpConnection,
    wsConnection,
    onImageFragment,
    onTdpError,
    onWsClose,
    onWsOpen,
    disconnected,
    setDisconnected,
    onKeyDown,
    onKeyUp,
    onMouseMove,
    onMouseDown,
    onMouseUp,
    onMouseWheelScroll,
  } = props;

  return (
    <Flex flexDirection="column">
      <TopBar
        onDisconnect={() => {
          setDisconnected(true);
          tdpClient.nuke();
        }}
        userHost={`${username}@${hostname}`}
        clipboard={clipboard}
        recording={recording}
      />

      <>
        {fetchAttempt.status === 'failed' && (
          <Alert
            style={{
              alignSelf: 'center',
            }}
            width={'450px'}
            my={2}
            children={fetchAttempt.statusText}
          />
        )}
        {tdpConnection.status === 'failed' && (
          <Alert
            style={{
              alignSelf: 'center',
            }}
            width={'450px'}
            my={2}
            children={tdpConnection.statusText}
          />
        )}
        {wsConnection === 'closed' &&
          tdpConnection.status !== 'failed' &&
          !disconnected &&
          tdpConnection.status !== 'processing' && (
            // If the websocket was closed for an unknown reason
            <Alert
              style={{
                alignSelf: 'center',
              }}
              width={'450px'}
              my={2}
              children={'Session disconnected for an unkown reason'}
            />
          )}

        {disconnected && (
          <Box textAlign="center" m={10}>
            <Text>Session successfully disconnected</Text>
          </Box>
        )}
        {(fetchAttempt.status === 'processing' ||
          tdpConnection.status === 'processing') && (
          <Box textAlign="center" m={10}>
            <Indicator />
          </Box>
        )}
      </>

      <TdpClientCanvas
        style={{
          display:
            fetchAttempt.status === 'success' &&
            tdpConnection.status === 'success' &&
            wsConnection === 'open' &&
            !disconnected
              ? 'flex'
              : 'none',
          flex: 1, // ensures the canvas fills available screen space
        }}
        tdpCli={tdpClient}
        tdpCliOnImageFragment={onImageFragment}
        tdpCliOnTdpError={onTdpError}
        tdpCliOnWsClose={onWsClose}
        tdpCliOnWsOpen={onWsOpen}
        onKeyDown={onKeyDown}
        onKeyUp={onKeyUp}
        onMouseMove={onMouseMove}
        onMouseDown={onMouseDown}
        onMouseUp={onMouseUp}
        onMouseWheelScroll={onMouseWheelScroll}
      />
    </Flex>
  );
}
