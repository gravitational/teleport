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
import styled, { useTheme } from 'styled-components';
import { Text, TopNav, Flex } from 'design';
import { Clipboard } from 'design/Icon';
import { colors } from 'teleport/Console/colors';
import ActionMenu from './ActionMenu';
import { DesktopSessionAttempt } from './useDesktopSession';

const RecordingIndicator = styled.div``;

export default function TopBar(props: Props) {
  const { userHost, clipboard, recording, attempt, onDisconnect } = props;
  const theme = useTheme();

  const noneUntilSuccess = () => {
    return {
      display: attempt.status === 'success' ? 'flex' : 'none',
    };
  };

  const showOnProcessing = () => {
    return {
      display: attempt.status === 'processing' ? 'flex' : 'none',
    };
  };

  const showOnFailure = () => {
    return {
      display: attempt.status === 'failed' ? 'flex' : 'none',
    };
  };

  const showOnDisconnected = () => {
    return {
      display: attempt.status === 'disconnected' ? 'flex' : 'none',
    };
  };

  // Used for centering the middle component in the TopBar in certain states
  const centeringDivStyle = () => {
    return {
      display: attempt.status !== 'success' ? 'flex' : 'none',
    };
  };

  const primaryOnTrue = (b: boolean): any => {
    return {
      color: b ? theme.colors.text.primary : theme.colors.text.secondary,
    };
  };

  const userHostStyle = {
    ...noneUntilSuccess(),
    color: theme.colors.text.secondary,
  };

  const clipboardTextStyle = {
    ...noneUntilSuccess(),
    ...primaryOnTrue(clipboard),
    verticalAlign: 'text-bottom',
  };

  const clipboardStyle = {
    ...primaryOnTrue(clipboard),
    fontWeight: theme.fontWeights.bold,
    fontSize: theme.fontSizes[4] + 'px',
    alignSelf: 'center',
  };

  const connectingStyle = {
    ...showOnProcessing(),
    color: theme.colors.text.secondary,
    textAlign: 'center',
  };

  const errorStyle = {
    ...showOnFailure(),
    color: theme.colors.text.secondary,
  };

  const disconnectedStyle = {
    ...showOnDisconnected(),
    color: theme.colors.text.secondary,
  };

  const recordingTextStyle = primaryOnTrue(recording);

  const recordingIndicatorStyle = {
    width: '10px',
    height: '10px',
    borderRadius: '10px',
    marginRight: '6px',
    backgroundColor: recording
      ? theme.colors.error.light
      : theme.colors.text.secondary,
    verticalAlign: 'text-bottom',
  };

  return (
    <TopNav
      height="56px"
      bg={colors.dark}
      style={{
        justifyContent: 'space-between',
      }}
    >
      <>
        <div style={centeringDivStyle()}></div>
        <Text px={3} style={userHostStyle}>
          {userHost}
        </Text>
      </>

      <>
        {/* Center element. Only one of these is shown at a time depending on attempt.status */}
        <Text style={clipboardTextStyle}>
          <Clipboard style={clipboardStyle} pr={2} />
          Clipboard Sharing {clipboard ? 'Enabled' : 'Disabled'}
        </Text>
        <Text style={connectingStyle}>Connecting...</Text>
        <Text style={errorStyle}>Error</Text>
        <Text style={disconnectedStyle}>Disconnected</Text>
      </>

      <>
        <div style={centeringDivStyle()}></div>
        <Flex px={3} style={noneUntilSuccess()}>
          <Flex alignItems="center">
            <RecordingIndicator style={recordingIndicatorStyle} />
            <Text style={recordingTextStyle}>
              {recording ? '' : 'Not '}Recording
            </Text>
          </Flex>
          <ActionMenu onDisconnect={onDisconnect} />
        </Flex>
      </>
    </TopNav>
  );
}

type Props = {
  userHost: string;
  clipboard: boolean;
  recording: boolean;
  attempt: DesktopSessionAttempt;
  onDisconnect: VoidFunction;
};
