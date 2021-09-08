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
import { Text, TopNav, Flex } from 'design';
import { Clipboard } from 'design/Icon';
import { colors } from 'teleport/Console/colors';
import ActionMenu from './ActionMenu';

type TopBarProps = {
  userHost: string;
  clipboard: boolean;
  recording: boolean;
};

const StyledUserHostText = styled(Text)`
  color: ${props => props.theme.colors.text.secondary};
`;

const StyledClipboardText = styled(Text)`
  color: ${props =>
    props.clipboard
      ? props.theme.colors.text.primary
      : props.theme.colors.text.secondary};
`;

// vertical-align: text-bottom makes the text appear vertically aligned with the center of the clipboard icon.
const StyledClipboard = styled(Clipboard)`
  color: ${props =>
    props.clipboard
      ? props.theme.colors.text.primary
      : props.theme.colors.text.secondary};
  font-weight: ${props => props.theme.fontWeights.bold};
  font-size: ${props => props.theme.fontSizes[4]}px;
  vertical-align: text-bottom;
`;

const StyledRecordingText = styled(Text)`
  color: ${props =>
    props.recording
      ? props.theme.colors.text.primary
      : props.theme.colors.text.secondary};
`;

const StyledRecordingIndicator = styled.div`
  width: 10px;
  height: 10px;
  border-radius: 10px;
  margin-right: 6px;
  background-color: ${props =>
    props.recording
      ? props.theme.colors.error.light
      : props.theme.colors.text.secondary};
  vertical-align: text-bottom;
`;

export default function TopBar(props: TopBarProps) {
  const { userHost, clipboard, recording } = props;
  return (
    <TopNav
      height="56px"
      bg={colors.dark}
      style={{
        justifyContent: 'space-between',
      }}
    >
      <StyledUserHostText px={3}>{userHost}</StyledUserHostText>
      <StyledClipboardText clipboard={clipboard}>
        <StyledClipboard clipboard={clipboard} /> Clipboard Sharing{' '}
        {clipboard ? 'Enabled' : 'Disabled'}
      </StyledClipboardText>
      <Flex px={3}>
        <Flex alignItems="center">
          <StyledRecordingIndicator recording={recording} />
          <StyledRecordingText recording={recording}>
            {recording ? '' : 'Not '}Recording
          </StyledRecordingText>
        </Flex>
        <ActionMenu
          onDisconnect={() => {
            console.log('TODO');
          }}
        />
      </Flex>
    </TopNav>
  );
}
