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
import ActionMenu from '../ActionMenu';

export default function TopBar(props: Props) {
  const { userHost, clipboard, recording, onDisconnect } = props;
  const theme = useTheme();

  const primaryOnTrue = (b: boolean): any => {
    return {
      color: b ? theme.colors.text.primary : theme.colors.text.secondary,
    };
  };

  return (
    <TopNav
      height={`${TopBarHeight}px`}
      bg={colors.dark}
      style={{
        justifyContent: 'space-between',
      }}
    >
      <Text px={3} style={{ color: theme.colors.text.secondary }}>
        {userHost}
      </Text>

      <Text
        style={{
          ...primaryOnTrue(clipboard),
          verticalAlign: 'text-bottom',
        }}
      >
        <StyledClipboard style={primaryOnTrue(clipboard)} pr={2} />
        Clipboard Sharing {clipboard ? 'Enabled' : 'Disabled'}
      </Text>

      <Flex px={3}>
        <Flex alignItems="center">
          <StyledRecordingIndicator
            style={{
              backgroundColor: recording
                ? theme.colors.error.light
                : theme.colors.text.secondary,
            }}
          />
          <Text style={primaryOnTrue(recording)}>
            {recording ? '' : 'Not '}Recording
          </Text>
        </Flex>
        <ActionMenu onDisconnect={onDisconnect} />
      </Flex>
    </TopNav>
  );
}

export const TopBarHeight = 40;

const StyledClipboard = styled(Clipboard)`
  font-weight: ${({ theme }) => theme.fontWeights.bold};
  font-size: ${({ theme }) => theme.fontSizes[4] + 'px'};
  align-self: 'center';
`;

const StyledRecordingIndicator = styled.div`
  width: 10px;
  height: 10px;
  border-radius: 10px;
  margin-right: 6px;
  vertical-align: text-bottom;
`;

type Props = {
  userHost: string;
  clipboard: boolean;
  recording: boolean;
  onDisconnect: VoidFunction;
};
