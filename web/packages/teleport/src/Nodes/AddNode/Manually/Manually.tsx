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
import { Text, Box } from 'design';
import TextSelectCopy from 'teleport/components/TextSelectCopy';
import DownloadLinks from 'teleport/components/DownloadLinks';

export default function Manually({
  isEnterprise,
  user,
  version,
  isAuthTypeLocal,
}: Props) {
  const { hostname, port } = window.document.location;
  const host = `${hostname}:${port || '443'}`;
  let tshLoginCmd = `tsh login --proxy=${host}`;

  if (isAuthTypeLocal) {
    tshLoginCmd = `${tshLoginCmd} --auth=local --user=${user}`;
  }

  return (
    <>
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
}

type Props = {
  isEnterprise: boolean;
  user: string;
  version: string;
  isAuthTypeLocal: boolean;
};
