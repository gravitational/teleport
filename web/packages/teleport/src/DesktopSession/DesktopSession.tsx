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

import React, { useEffect } from 'react';
import useDesktopSession, { State } from './useDesktopSession';
import TopBar from './TdpClientCanvas/TopBar';
import { Indicator, Box, Alert, Text, Flex } from 'design';
import useTeleport from 'teleport/useTeleport';
import TdpClientCanvas from './TdpClientCanvas';
import useAttempt from 'shared/hooks/useAttemptNext';

export default function Container() {
  const ctx = useTeleport();
  const state = useDesktopSession(ctx);
  return <DesktopSession {...state} />;
}

export function DesktopSession(props: State) {
  const {
    hostname,
    clipboard,
    recording,
    tdpClient,
    fetchAttempt,
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
  } = props;

  const { attempt, setAttempt } = useAttempt('processing'); // attempt.status === '' means disconnected

  // Sets attempt based on combination of fetchAttempt and tdpclient connection
  useEffect(() => {
    if (fetchAttempt.status === 'failed') {
      setAttempt(fetchAttempt);
    } else if (connectionAttempt.status === 'failed') {
      setAttempt(connectionAttempt);
    } else if (connectionAttempt.status === '') {
      setAttempt(connectionAttempt);
    } else if (
      fetchAttempt.status === 'processing' ||
      connectionAttempt.status === 'processing'
    ) {
      setAttempt({ status: 'processing' });
    } else if (
      fetchAttempt.status === 'success' &&
      connectionAttempt.status === 'success'
    ) {
      setAttempt(connectionAttempt);
    } else {
      setAttempt({ status: 'failed', statusText: 'unknown error' });
    }
  }, [fetchAttempt, connectionAttempt]);

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

      <>
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
        {attempt.status === '' && (
          <Box textAlign="center" m={10}>
            <Text>Session successfully disconnected</Text>
          </Box>
        )}
        {attempt.status === 'processing' && (
          <Box textAlign="center" m={10}>
            <Indicator />
          </Box>
        )}
      </>

      <TdpClientCanvas
        style={{
          display: attempt.status === 'success' ? 'flex' : 'none',
          flex: 1,
        }}
        tdpClient={tdpClient}
        connectionAttempt={connectionAttempt}
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
        onMouseWheelScroll={onMouseWheelScroll}
      />
    </Flex>
  );
}
