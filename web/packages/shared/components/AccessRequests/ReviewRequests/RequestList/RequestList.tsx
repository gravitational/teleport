/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { Flex, LabelState, Text } from 'design';
import { Cell } from 'design/DataTable';
import { ArrowFatLinesUp } from 'design/Icon';
import { LabelKind } from 'design/LabelState/LabelState';
import { AccessRequest } from 'shared/services/accessRequests';

export const renderUserCell = ({ user }: AccessRequest) => {
  return (
    <Cell
      style={{
        maxWidth: '100px',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
      }}
      title={user}
    >
      {user}
    </Cell>
  );
};

export const renderIdCell = ({ id }: AccessRequest) => {
  return (
    <Cell
      style={{
        maxWidth: '100px',
        whiteSpace: 'nowrap',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
      }}
      title={id}
    >
      {id.slice(-5)}
    </Cell>
  );
};

export const renderStatusCell = ({ state }: AccessRequest) => {
  if (state === 'PROMOTED') {
    return (
      <Cell>
        <Flex alignItems="center">
          <ArrowFatLinesUp size={17} color="success.main" mr={1} ml="-3px" />
          <Text typography="body3">{state}</Text>
        </Flex>
      </Cell>
    );
  }

  let kind: LabelKind = 'warning';
  if (state === 'APPROVED') {
    kind = 'success';
  } else if (state === 'DENIED') {
    kind = 'danger';
  }

  return (
    <Cell>
      <Flex alignItems="center">
        <LabelState
          kind={kind}
          mr={2}
          width="10px"
          p={0}
          style={{ minHeight: '10px' }}
        />
        <Text typography="body3">{state}</Text>
      </Flex>
    </Cell>
  );
};
