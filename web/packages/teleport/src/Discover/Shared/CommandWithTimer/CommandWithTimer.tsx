// Copyright 2022 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import React from 'react';
import styled from 'styled-components';
import { Text, Box } from 'design';
import * as Icons from 'design/Icon';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import { Timeout } from 'teleport/Discover/Shared/Timeout';
import { TextIcon } from 'teleport/Discover/Shared';

export const CommandWithTimer = ({
  command,
  poll,
  pollingTimeout,
  header,
}: Props) => {
  return (
    <PollBox p={3} borderRadius={3} pollState={poll.state}>
      {header || <Text bold>Command</Text>}
      <Box mt={2} mb={1}>
        <TextSelectCopyMulti lines={[{ text: command }]} />
      </Box>
      {poll.state === 'polling' && (
        <TextIcon
          css={`
            white-space: pre;
          `}
        >
          <Icons.Restore fontSize={4} />
          <Timeout
            timeout={pollingTimeout}
            message={`${
              poll.customStateDesc || 'Waiting for Teleport Service'
            }  |  `}
          />
        </TextIcon>
      )}
      {poll.state === 'success' && (
        <TextIcon>
          <Icons.CircleCheck ml={1} color="success" />
          {poll.customStateDesc ||
            'The Teleport Service successfully join this Teleport cluster'}
        </TextIcon>
      )}
      {poll.state === 'error' && (
        <Box>
          <TextIcon>
            <Icons.Warning ml={1} color="danger" />
            {poll.error.customErrContent ||
              'We could not detect the Teleport Service you were trying to add'}
          </TextIcon>
          <Text bold mt={4}>
            Possible reasons
          </Text>
          <ul
            css={`
              margin-top: 6px;
              margin-bottom: 0;
            `}
          >
            {poll.error.reasonContents.map((content, index) => (
              <li key={index}>{content}</li>
            ))}
          </ul>
        </Box>
      )}
    </PollBox>
  );
};

type PollError = {
  reasonContents: React.ReactNode[];
  customErrContent?: React.ReactNode;
};

export type PollState = 'polling' | 'success' | 'error';

export type Poll = {
  state: 'polling' | 'success' | 'error';
  customStateDesc?: string;
  // error only needs to be defined when
  // poll state is 'error'.
  error?: PollError;
};

export const PollBox = styled(Box)`
  max-width: 800px;
  background-color: rgba(255, 255, 255, 0.05);
  padding: ${props => `${props.theme.space[3]}px`};
  border-radius: ${props => `${props.theme.space[2]}px`};
  border: 2px solid
    ${props => {
      switch (props.pollState) {
        case 'error':
          return props.theme.colors.danger;
        case 'success':
          return props.theme.colors.success;
        default:
          // polling
          return '#2F3659';
      }
    }};
`;

export type Props = {
  command: string;
  poll: Poll;
  pollingTimeout: number;
  header?: React.ReactNode;
};
