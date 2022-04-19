/**
 * Copyright 2020 Gravitational, Inc.
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

import React, { useEffect } from 'react';
import { Text, Box, ButtonLink, Indicator, ButtonSecondary } from 'design';
import TextSelectCopy from 'teleport/components/TextSelectCopy';
import DownloadLinks from 'teleport/components/DownloadLinks';
import { State } from './../useAddNode';
import { DialogContent, DialogFooter } from 'design/Dialog';

export default function Manually({
  isEnterprise,
  user,
  version,
  isAuthTypeLocal,
  joinToken,
  createJoinToken,
  attempt,
  onClose,
}: Props) {
  const { hostname, port } = window.document.location;
  const host = `${hostname}:${port || '443'}`;
  let tshLoginCmd = `tsh login --proxy=${host}`;

  useEffect(() => {
    if (!joinToken) {
      createJoinToken();
    }
  }, []);

  if (isAuthTypeLocal) {
    tshLoginCmd = `${tshLoginCmd} --auth=local --user=${user}`;
  }

  if (attempt.status === 'processing' || attempt.status === '') {
    return (
      <Box textAlign="center">
        <Indicator />
      </Box>
    );
  }

  return (
    <>
      <DialogContent>
        <Box mb={4}>
          <Text bold as="span">
            Step 1
          </Text>{' '}
          - Download Teleport package to your computer
          <DownloadLinks isEnterprise={isEnterprise} version={version} />
        </Box>
        {attempt.status === 'failed' ? (
          <StepsWithoutToken host={host} tshLoginCmd={tshLoginCmd} />
        ) : (
          <StepsWithToken
            joinToken={joinToken}
            host={host}
            createJoinToken={createJoinToken}
          />
        )}
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </>
  );
}

type StepsWithoutTokenProps = {
  tshLoginCmd: string;
  host: string;
};

const StepsWithoutToken = ({ tshLoginCmd, host }: StepsWithoutTokenProps) => (
  <>
    <Box mb={4}>
      <Text bold as="span">
        Step 2
      </Text>
      {' - Login to Teleport'}
      <TextSelectCopy mt="2" text={tshLoginCmd} />
    </Box>
    <Box mb={4}>
      <Text bold as="span">
        Step 3
      </Text>
      {' - Generate a join token'}
      <TextSelectCopy mt="2" text="tctl tokens add --type=node --ttl=1h" />
    </Box>
    <Box>
      <Text bold as="span">
        Step 4
      </Text>
      {` - Start the Teleport agent with the following parameters`}
      <TextSelectCopy
        mt="2"
        text={`teleport start --roles=node --token=[generated-join-token] --auth-server=${host} `}
      />
    </Box>
  </>
);

type StepsWithTokenProps = {
  joinToken: State['token'];
  host: string;
  createJoinToken: State['createJoinToken'];
};

const StepsWithToken = ({
  joinToken,
  host,
  createJoinToken,
}: StepsWithTokenProps) => (
  <Box>
    <Text bold as="span">
      Step 2
    </Text>
    {` - Start the Teleport agent with the following parameters`}
    <Text mt="1">
      The token will be valid for{' '}
      <Text bold as={'span'}>
        {joinToken.expiryText}.
      </Text>
    </Text>
    <TextSelectCopy
      mt="2"
      text={`teleport start --roles=node --token=${joinToken.id} --auth-server=${host} `}
    />
    <Box>
      <ButtonLink onClick={createJoinToken}>Regenerate Token</ButtonLink>
    </Box>
  </Box>
);

type Props = {
  isEnterprise: boolean;
  user: string;
  version: string;
  isAuthTypeLocal: boolean;
  joinToken: State['token'];
  createJoinToken: State['createJoinToken'];
  attempt: State['attempt'];
  onClose(): void;
};
