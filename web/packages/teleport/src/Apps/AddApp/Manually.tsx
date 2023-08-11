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

import React from 'react';
import {
  Text,
  Box,
  ButtonSecondary,
  Link,
  Indicator,
  ButtonLink,
} from 'design';
import { DialogContent, DialogFooter } from 'design/Dialog';

import TextSelectCopy from 'teleport/components/TextSelectCopy';
import DownloadLinks from 'teleport/components/DownloadLinks';
import cfg from 'teleport/config';

import { State } from './useAddApp';

export function Manually({
  isEnterprise,
  user,
  version,
  onClose,
  isAuthTypeLocal,
  token,
  createToken,
  attempt,
}: Props) {
  const { hostname, port } = window.document.location;
  const host = `${hostname}:${port || '443'}`;
  let tshLoginCmd = `tsh login --proxy=${host}`;

  if (isAuthTypeLocal) {
    tshLoginCmd = `${tshLoginCmd} --auth=local --user=${user}`;
  }

  if (attempt.status === 'processing') {
    return (
      <Box textAlign="center">
        <Indicator />
      </Box>
    );
  }

  return (
    <>
      <DialogContent flex="0 0 auto">
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
          <StepsWithToken createToken={createToken} host={host} token={token} />
        )}
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </>
  );
}

const configFile = `${cfg.configDir}/app_config.yaml`;
const startCmd = `teleport start --config=${configFile}`;

function getConfigCmd(token: string, host: string) {
  return `teleport configure --output=${configFile} --app-name=[example-app] --app-uri=http://localhost/ \
--roles=app --token=${token} --proxy=${host} --data-dir=${cfg.configDir}`;
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
      <TextSelectCopy mt="2" text="tctl tokens add --type=app" />
    </Box>
    <Box mb="4">
      <Text bold as="span">
        Step 4
      </Text>
      {` - Configure your teleport agent`}
      <TextSelectCopy
        mt="2"
        text={getConfigCmd('[generated-join-token]', host)}
      />
    </Box>
    <Box>
      <Text bold as="span">
        Step 5
      </Text>
      {` - Start the Teleport agent with the generated configuration file`}
      <TextSelectCopy mt="2" text={startCmd} />
    </Box>
    <Box>
      {`* Note: For a self-hosted Teleport version, you may need to update DNS and obtain a TLS certificate for this application.
            Learn more about application access `}
      <Link
        href={'https://goteleport.com/docs/application-access/introduction/'}
        target="_blank"
      >
        here
      </Link>
      .
    </Box>
  </>
);

type StepsWithTokenProps = {
  token: State['token'];
  host: string;
  createToken: State['createToken'];
};

const StepsWithToken = ({ token, host, createToken }: StepsWithTokenProps) => (
  <>
    <Box mb={4}>
      <Text bold as="span">
        Step 2
      </Text>
      {` - Configure your teleport agent`}
      <Text mt="1">
        The token will be valid for{' '}
        <Text bold as={'span'}>
          {token.expiryText}.
        </Text>
      </Text>
      <TextSelectCopy mt="2" text={getConfigCmd(token.id, host)} />
      <Box>
        <ButtonLink onClick={createToken}>Regenerate Token</ButtonLink>
      </Box>
    </Box>
    <Box>
      <Text bold as="span">
        Step 3
      </Text>
      {` - Start the Teleport agent with the configuration file`}
      <TextSelectCopy mt="2" text={startCmd} />
    </Box>
  </>
);

type Props = {
  onClose(): void;
  isEnterprise: boolean;
  version: string;
  user: string;
  isAuthTypeLocal: boolean;
  token: State['token'];
  createToken: State['createToken'];
  attempt: State['attempt'];
};
