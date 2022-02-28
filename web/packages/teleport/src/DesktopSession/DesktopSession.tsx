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

import React, { PropsWithChildren } from 'react';
import styled from 'styled-components';
import { Indicator, Box, Alert, Text, Flex } from 'design';
import TdpClientCanvas from 'teleport/components/TdpClientCanvas';
import AuthnDialog from 'teleport/components/AuthnDialog';
import useDesktopSession, { State } from './useDesktopSession';
import TopBar from './TopBar';

export default function Container() {
  const state = useDesktopSession();
  return <DesktopSession {...state} />;
}

export function DesktopSession(props: State) {
  const {
    clipboardState,
    fetchAttempt,
    tdpConnection,
    wsConnection,
    disconnected,
  } = props;

  const clipboardError = clipboardState.enabled && clipboardState.errorText;

  const clipboardProcessing =
    clipboardState.enabled && clipboardState.permission.state === 'prompt';

  // Websocket is closed but we haven't
  // closed it on purpose or registered a tdp error.
  const unknownConnectionError =
    wsConnection === 'closed' &&
    !disconnected &&
    tdpConnection.status === 'success';

  const processing =
    fetchAttempt.status === 'processing' ||
    tdpConnection.status === 'processing' ||
    clipboardProcessing;

  let alertText: string;
  if (fetchAttempt.status === 'failed') {
    alertText = fetchAttempt.statusText || 'fetch attempt failed';
  } else if (tdpConnection.status === 'failed') {
    alertText = tdpConnection.statusText || 'tdp connection failed';
  } else if (clipboardError) {
    alertText = clipboardState.errorText || 'clipboard sharing failed';
  } else if (unknownConnectionError) {
    alertText = 'Session disconnected for an unknown reason';
  } else {
    alertText = '';
  }

  if (alertText) {
    return (
      <Session {...props}>
        <DesktopSessionAlert my={2} children={alertText} />
      </Session>
    );
  }

  if (disconnected) {
    return (
      <Session {...props}>
        <Box textAlign="center" m={10}>
          <Text>Session successfully disconnected</Text>
        </Box>
      </Session>
    );
  }

  if (processing) {
    return (
      <Session {...props}>
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      </Session>
    );
  }

  return <Session {...props}></Session>;
}

function Session(props: PropsWithChildren<State>) {
  const {
    fetchAttempt,
    tdpConnection,
    wsConnection,
    disconnected,
    setDisconnected,
    webauthn,
    tdpClient,
    username,
    hostname,
    clipboardState,
    isRecording,
    onPngFrame,
    onClipboardData,
    onTdpError,
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
  } = props;

  const clipboardSharingActive =
    clipboardState.enabled && clipboardState.permission.state === 'granted';
  const clipboardSuccess =
    !clipboardState.enabled ||
    (clipboardState.enabled &&
      clipboardState.permission.state === 'granted' &&
      clipboardState.errorText === '');

  const showCanvas =
    fetchAttempt.status === 'success' &&
    tdpConnection.status === 'success' &&
    wsConnection === 'open' &&
    !disconnected &&
    clipboardSuccess;

  return (
    <Flex flexDirection="column">
      <TopBar
        onDisconnect={() => {
          setDisconnected(true);
          tdpClient.nuke();
        }}
        userHost={`${username}@${hostname}`}
        clipboard={clipboardSharingActive}
        recording={isRecording}
      />

      {props.children}

      {webauthn.requested && (
        <AuthnDialog
          onContinue={webauthn.authenticate}
          onCancel={() => {
            webauthn.setState(prevState => {
              return {
                ...prevState,
                errorText:
                  'This session requires multi factor authentication to continue. Please hit "Retry" and follow the prompts given by your browser to complete authentication.',
              };
            });
          }}
          errorText={webauthn.errorText}
        />
      )}

      {/* TdpClientCanvas should always be present in the DOM so that it calls
          tdpClient.init() and initializes the required tdpClient event listeners,
          both of which are needed for this component's state to properly respond to
          initialization events. */}
      <TdpClientCanvas
        style={{
          display: showCanvas ? 'flex' : 'none',
          flex: 1, // ensures the canvas fills available screen space
        }}
        tdpCli={tdpClient}
        tdpCliOnPngFrame={onPngFrame}
        tdpCliOnClipboardData={onClipboardData}
        tdpCliOnTdpError={onTdpError}
        tdpCliOnWsClose={onWsClose}
        tdpCliOnWsOpen={onWsOpen}
        onKeyDown={onKeyDown}
        onKeyUp={onKeyUp}
        onMouseMove={onMouseMove}
        onMouseDown={onMouseDown}
        onMouseUp={onMouseUp}
        onMouseWheelScroll={onMouseWheelScroll}
        onContextMenu={onContextMenu}
        onMouseEnter={onMouseEnter}
        windowOnFocus={windowOnFocus}
      />
    </Flex>
  );
}

const DesktopSessionAlert = styled(Alert)`
  align-self: center;
  min-width: 450px;
`;
