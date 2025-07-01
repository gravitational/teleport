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

import { styled, useTheme } from 'styled-components';

import Flex from 'design/Flex';
import { Stars } from 'design/Icon/Icons/Stars';
import Label from 'design/Label/Label';
import { fontWeights } from 'design/theme/typography';
import { traitsPreset } from 'shared/components/TraitsEditor/TraitsEditor';

import { ApiBotTrait } from 'teleport/services/bot/types';

import { Panel } from './Panel';

export function Traits(props: { traits: ApiBotTrait[] }) {
  const { traits } = props;

  const theme = useTheme();

  return (
    <Panel title="Traits" testId="traits-panel">
      <TransposedTable>
        <tbody>
          {traits
            .sort((a, b) => a.name.localeCompare(b.name))
            .map(r => (
              <tr key={r.name}>
                <th scope="row">
                  <Flex gap={2}>
                    {traitsPreset.includes(r.name) ? (
                      <Stars
                        size={'small'}
                        color={theme.colors.interactive.solid.primary.default}
                      />
                    ) : undefined}
                    {r.name}
                  </Flex>
                </th>
                <td>
                  {r.values.length > 0
                    ? r.values.map(v => (
                        <Label mr="1" key={v} kind="outline">
                          {v}
                        </Label>
                      ))
                    : 'no values'}
                </td>
              </tr>
            ))}
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
