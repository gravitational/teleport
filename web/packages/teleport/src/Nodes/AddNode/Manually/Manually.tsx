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
import { Text, Box, Link } from 'design';
import TextSelectCopy from 'teleport/components/TextSelectCopy';
import * as links from 'teleport/services/links';
import cfg from 'teleport/config';

export default function Manually({ version, ...styles }: Props) {
  return (
    <Box {...styles}>
      <Box mb={4}>
        <Text bold as="span">
          Step 1
        </Text>{' '}
        - Download Teleport package to your computer
        <Box>
          <Link href={links.getMacOS(version)} target="_blank" mr="2">
            MacOS
          </Link>
          <Link href={links.getLinux64(version)} target="_blank" mr="2">
            Linux 64-bit
          </Link>
          <Link href={links.getLinux32(version)} target="_blank">
            Linux 32-bit
          </Link>
        </Box>
      </Box>
      <Box mb={4}>
        <Text bold as="span">
          Step 2
        </Text>
        {' - Login to Teleport'}
        <TextSelectCopy
          mt="2"
          text={`tsh login --proxy=${cfg.proxyCluster} --auth=local`}
        />
      </Box>
      <Box mb={4}>
        <Text bold as="span">
          Step 3
        </Text>
        {' - Generate a join token'}
        <TextSelectCopy mt="2" text="tctl tokens add --type=node" />
      </Box>
      <Box>
        <Text bold as="span">
          Step 4
        </Text>
        {` - Install Teleport on target server, and start it with the following parameters`}
        <TextSelectCopy
          mt="2"
          text={`teleport start --roles=node --token=<generated-node-join-token> --auth-server=${cfg.proxyCluster} `}
        />
      </Box>
    </Box>
  );
}

type Props = {
  version: string;
  // handles styles
  [key: string]: any;
};
