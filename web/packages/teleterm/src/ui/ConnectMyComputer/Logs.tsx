/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { Flex, Text } from 'design';
import React from 'react';

interface LogsProps {
  logs: string;
}

export function Logs(props: LogsProps): JSX.Element {
  return (
    <>
      <Text mb={2}>Last 10 lines of logs:</Text>
      <Flex
        width="100%"
        color="light"
        bg="bgTerminal"
        p={2}
        mb={2}
        flexDirection="column"
        borderRadius={1}
      >
        <span
          css={`
            white-space: pre-wrap;
            font-size: 12px;
            font-family: ${props => props.theme.fonts.mono};
          `}
        >
          {props.logs}
        </span>
      </Flex>
    </>
  );
}
