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

import React from 'react';
import { Danger } from 'design/Alert';
import { Flex, Text, ButtonPrimary } from 'design';

export function Reconnect(props: {
  kubeId: string;
  statusText: string;
  reconnect: () => void;
}) {
  return (
    <Flex gap={4} flexDirection="column" mx="auto" alignItems="center" mt={100}>
      <Text typography="h5" color="text.main">
        A connection to <strong>{props.kubeId}</strong> has failed.
      </Text>
      <Flex flexDirection="column" alignItems="center" mx="auto">
        <Danger mb={3}>
          <Text textAlign="center" css={'white-space: pre-wrap;'}>
            {props.statusText}
          </Text>
        </Danger>
        <ButtonPrimary width="100px" onClick={props.reconnect}>
          Retry
        </ButtonPrimary>
      </Flex>
    </Flex>
  );
}
