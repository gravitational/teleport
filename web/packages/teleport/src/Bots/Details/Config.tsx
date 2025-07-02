/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import styled from 'styled-components';

import Flex from 'design/Flex/Flex';
import Text from 'design/Text';
import { fontWeights } from 'design/theme/typography';
import { CopyButton } from 'shared/components/UnifiedResources/shared/CopyButton';

import { formatDuration } from '../formatDuration';
import { Panel } from './Panel';

const botNameLabel = 'Bot name';
const maxSessionDurationLabel = 'Max session duration';

export function Config(props: {
  botName: string;
  maxSessionDurationSeconds?: number;
}) {
  const { botName, maxSessionDurationSeconds } = props;

  return (
    <Panel title="Metadata" isSubPanel testId="config-panel">
      <TransposedTable>
        <tbody>
          <tr>
            <th scope="row">{botNameLabel}</th>
            <td>
              <Flex inline alignItems={'center'} gap={1} mr={0}>
                <MonoText>{botName}</MonoText>
                <CopyButton name={botName} />
              </Flex>
            </td>
          </tr>
          <tr>
            <th scope="row">{maxSessionDurationLabel}</th>
            <td>
              {maxSessionDurationSeconds
                ? formatDuration(maxSessionDurationSeconds)
                : '-'}
            </td>
          </tr>
        </tbody>
      </TransposedTable>
    </Panel>
  );
}

const TransposedTable = styled.table`
  th {
    text-align: start;
    padding-right: 16px;
    width: 1%; // Minimum width to fit content
    color: ${({ theme }) => theme.colors.text.muted};
    font-weight: ${fontWeights.regular};
  }
`;

const MonoText = styled(Text)`
  font-family: ${({ theme }) => theme.fonts.mono};
`;
