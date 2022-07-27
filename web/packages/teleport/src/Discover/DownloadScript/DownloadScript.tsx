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
import { Text, Box, Indicator } from 'design';
import { Danger } from 'design/Alert';

import cfg from 'teleport/config';
import TextSelectCopy from 'teleport/components/TextSelectCopy';

import { useDiscoverContext } from '../discoverContextProvider';
import { AgentStepProps } from '../types';

import { Header, ActionButtons } from '../Shared';

import { useDownloadScript } from './useDownloadScript';

import type { State } from './useDownloadScript';

export default function Container(props: AgentStepProps) {
  const ctx = useDiscoverContext();
  const state = useDownloadScript({ ctx, props });

  return <DownloadScript {...state} />;
}

export function DownloadScript({ attempt, joinToken, nextStep }: State) {
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
          <Text>
            Use below script to add a server to your cluster. This script will
            install the Teleport agent to provide secure access to your server.
            <Text mt="3">
              The script will be valid for{' '}
              <Text bold as={'span'}>
                {joinToken.expiryText}.
              </Text>
            </Text>
          </Text>
          <TextSelectCopy
            text={createBashCommand(joinToken.id)}
            mt={2}
            maxWidth="800px"
          />
          <ActionButtons onProceed={nextStep} />
        </>
      )}
    </Box>
  );
}

function createBashCommand(tokenId: string) {
  return `sudo bash -c "$(curl -fsSL ${cfg.getNodeScriptUrl(tokenId)})"`;
}
