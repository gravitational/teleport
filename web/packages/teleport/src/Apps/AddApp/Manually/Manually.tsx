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
import { Text, Box, ButtonSecondary, Link } from 'design';
import { DialogContent, DialogFooter } from 'design/Dialog';
import TextSelectCopy from 'teleport/components/TextSelectCopy';
import DownloadLinks from 'teleport/components/DownloadLinks';
import useTeleport from 'teleport/useTeleport';

export default function Manually({
  user,
  version,
  onClose,
  isAuthTypeLocal,
}: Props) {
  const { hostname, port } = window.document.location;
  const host = `${hostname}:${port || '443'}`;
  let tshLoginCmd = `tsh login --proxy=${host}`;

  const ctx = useTeleport();
  const isEnterprise = ctx.isEnterprise;

  if (isAuthTypeLocal) {
    tshLoginCmd = `${tshLoginCmd} --auth=local --user=${user}`;
  }

  return (
    <>
      <DialogContent minHeight="240px" flex="0 0 auto">
        <Box mb={4}>
          <Text bold as="span">
            Step 1
          </Text>{' '}
          - Download Teleport package to your computer
          <DownloadLinks isEnterprise={isEnterprise} version={version} />
        </Box>
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
          {` - Start the Teleport agent with the following parameters`}
          <TextSelectCopy
            mt="2"
            text={`teleport start --roles=app --app-name=[example-app] --app-uri=http://localhost/ --token=[generated-join-token] --auth-server=${host}`}
          />
        </Box>
        <Box>
          {`* Note: For a self-hosted Teleport version, you may need to update DNS and obtain a TLS certificate for this application.
            Learn more about application access `}
          <Link
            href={'https://goteleport.com/teleport/docs/application-access/'}
            target="_blank"
          >
            here
          </Link>
          .
        </Box>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </>
  );
}

type Props = {
  onClose(): void;
  version: string;
  user: string;
  isAuthTypeLocal: boolean;
};
