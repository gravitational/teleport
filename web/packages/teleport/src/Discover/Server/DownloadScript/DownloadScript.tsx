/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled from 'styled-components';
import { Text, Box, Indicator } from 'design';
import * as Icons from 'design/Icon';

import cfg from 'teleport/config';
import TextSelectCopy from 'teleport/components/TextSelectCopy';
import useTeleport from 'teleport/useTeleport';

import { AgentStepProps } from '../../types';

import {
  ActionButtons,
  ButtonBlueText,
  Header,
  HeaderSubtitle,
  Mark,
  TextIcon,
} from '../../Shared';

import { useDownloadScript } from './useDownloadScript';

import type { State, CountdownTime } from './useDownloadScript';

export default function Container(props: AgentStepProps) {
  const ctx = useTeleport();
  const state = useDownloadScript({ ctx, props });

  return <DownloadScript {...state} />;
}

export function DownloadScript({
  attempt,
  joinToken,
  nextStep,
  pollState,
  regenerateScriptAndRepoll,
  countdownTime,
}: State) {
  return (
    <Box>
      <Header>Configure Resource</Header>
      <HeaderSubtitle>
        Install and configure the Teleport SSH Service.
        <br />
        Run the following command on the server you want to add.
      </HeaderSubtitle>
      <ScriptBox
        p={3}
        borderRadius={3}
        pollState={attempt.status === 'failed' ? 'error' : pollState}
        height={attempt.status === 'processing' ? '144px' : 'auto'}
      >
        <Text bold>Command</Text>
        {attempt.status === 'processing' && (
          <Box textAlign="center" height="108px">
            <Indicator />
          </Box>
        )}
        {attempt.status === 'failed' && (
          <>
            <TextIcon mt={2} mb={3}>
              <Icons.Warning ml={1} color="danger" />
              Encountered Error: {attempt.statusText}
            </TextIcon>
            <ButtonBlueText ml={2} onClick={regenerateScriptAndRepoll}>
              Refetch a command
            </ButtonBlueText>
          </>
        )}
        {attempt.status === 'success' && (
          <>
            <TextSelectCopy
              text={createBashCommand(joinToken.id)}
              mt={2}
              mb={1}
            />
            {pollState === 'polling' && (
              <TextIcon
                css={`
                  white-space: pre;
                `}
              >
                <Icons.Restore fontSize={4} />
                {`Waiting for Teleport SSH Service   |   ${formatTime(
                  countdownTime
                )}`}
              </TextIcon>
            )}
            {pollState === 'success' && (
              <TextIcon>
                <Icons.CircleCheck ml={1} color="success" />
                The server successfully joined this Teleport cluster
              </TextIcon>
            )}
            {pollState === 'error' && (
              <TimeoutError
                regenerateScriptAndRepoll={regenerateScriptAndRepoll}
              />
            )}
          </>
        )}
      </ScriptBox>
      <ActionButtons
        onProceed={nextStep}
        disableProceed={
          pollState === 'error' ||
          pollState === 'polling' ||
          attempt.status === 'processing' ||
          attempt.status === 'failed'
        }
      />
    </Box>
  );
}

function createBashCommand(tokenId: string) {
  return `sudo bash -c "$(curl -fsSL ${cfg.getNodeScriptUrl(tokenId)})"`;
}

function formatTime({ minutes, seconds }: CountdownTime) {
  const formattedSeconds = String(seconds).padStart(2, '0');
  const formattedMinutes = String(minutes).padStart(2, '0');

  let timeNotation = 'minute';
  if (!minutes && seconds >= 0) {
    timeNotation = 'seconds';
  }
  if (minutes) {
    timeNotation = 'minutes';
  }

  return `${formattedMinutes}:${formattedSeconds} ${timeNotation}`;
}

const ScriptBox = styled(Box)`
  max-width: 800px;
  background-color: rgba(255, 255, 255, 0.05);
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

const TimeoutError = ({
  regenerateScriptAndRepoll,
}: {
  regenerateScriptAndRepoll(): void;
}) => {
  return (
    <Box>
      <TextIcon>
        <Icons.Warning ml={1} color="danger" />
        We could not detect the server you were trying to add{' '}
        <ButtonBlueText ml={1} onClick={regenerateScriptAndRepoll}>
          Generate a new command
        </ButtonBlueText>
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
        <li>The command was not run on the server you were trying to add</li>
        <li>
          The Teleport SSH Service could not join this Teleport cluster. Check
          the logs for errors by running <br />
          <Mark>journalctl status teleport</Mark>
        </li>
      </ul>
    </Box>
  );
};
