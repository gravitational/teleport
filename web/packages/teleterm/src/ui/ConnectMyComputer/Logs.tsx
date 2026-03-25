/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import type { JSX } from 'react';

import { Flex, Text } from 'design';

interface LogsProps {
  logs: string;
}

export function Logs(props: LogsProps): JSX.Element {
  return (
    <Flex flexDirection="column" gap={1}>
      <Text>Last 10 lines of logs:</Text>
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
    </Flex>
  );
}
