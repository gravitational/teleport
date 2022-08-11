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
import { Text, Box, Indicator, ButtonText } from 'design';
import * as Icons from 'design/Icon';
import { Danger } from 'design/Alert';

import cfg from 'teleport/config';
import TextSelectCopy from 'teleport/components/TextSelectCopy';

import { useDiscoverContext } from '../discoverContextProvider';
import { AgentStepProps } from '../types';

import { Header, ActionButtons, TextIcon } from '../Shared';

import { useDownloadScript } from './useDownloadScript';

import type { State } from './useDownloadScript';

export default function Container(props: AgentStepProps) {
  const ctx = useDiscoverContext();
  const state = useDownloadScript({ ctx, props });

  return <DownloadScript {...state} />;
}

export function DownloadScript({
  attempt,
  joinToken,
  nextStep,
  pollState,
  regenerateScriptAndRepoll,
}: State) {
  return (
    <Box>
      <Header>Configure Resource</Header>
      {attempt.status === 'failed' && <Danger>{attempt.statusText}</Danger>}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'success' && (
        <>
          <Text mb={3}>
            Use below script to add a server to your cluster. This script will
            install the Teleport agent to provide secure access to your server.
          </Text>
          <ScriptBox p={3} borderRadius={3} pollState={pollState}>
            <Text bold>Script</Text>
            <TextSelectCopy
              text={createBashCommand(joinToken.id)}
              mt={2}
              mb={1}
            />

            {pollState === 'polling' && (
              <TextIcon>
                <Icons.Restore fontSize={4} />
                Waiting for resource discovery...
              </TextIcon>
            )}
            {pollState === 'success' && (
              <TextIcon>
                <Icons.CircleCheck ml={1} color="success" />
                Successfully discovered resource
              </TextIcon>
            )}
            {pollState === 'error' && (
              <TextIcon>
                <Icons.Warning ml={1} color="danger" />
                Timed out, failed to discover resource.{' '}
                <ButtonText
                  onClick={regenerateScriptAndRepoll}
                  css={`
                    color: ${({ theme }) => theme.colors.link};
                    font-weight: normal;
                    padding-left: 2px;
                    font-size: inherit;
                    min-height: auto;
                  `}
                >
                  Generate a new script and try again.
                </ButtonText>
              </TextIcon>
            )}
          </ScriptBox>
          <ActionButtons
            onProceed={nextStep}
            disableProceed={pollState === 'error' || pollState === 'polling'}
          />
        </>
      )}
    </Box>
  );
}

function createBashCommand(tokenId: string) {
  return `sudo bash -c "$(curl -fsSL ${cfg.getNodeScriptUrl(tokenId)})"`;
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
