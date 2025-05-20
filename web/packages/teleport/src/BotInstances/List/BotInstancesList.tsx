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

import formatDistanceToNowStrict from 'date-fns/formatDistanceToNowStrict';
import styled from 'styled-components';

import { Info } from 'design/Alert/Alert';
import { Cell, LabelCell } from 'design/DataTable/Cells';
import Flex from 'design/Flex';
import Text from 'design/Text';
import { CopyButton } from 'shared/components/UnifiedResources/shared/CopyButton';

import { ClientSearcheableTableWithQueryParamSupport } from 'teleport/components/ClientSearcheableTableWithQueryParamSupport/ClientSearcheableTableWithQueryParamSupport';
import { BotInstanceSummary } from 'teleport/services/bot/types';

const MonoText = styled(Text)`
  font-family: ${({ theme }) => theme.fonts.mono};
`;

export function BotInstancesList({
  data,
  pageSize = 20,
}: {
  data: BotInstanceSummary[];
  pageSize?: number;
}) {
  const tableData = data.map(x => ({
    ...x,
    hostnameDisplay: x.host_name_latest ?? '-',
    shortInstanceId: x.instance_id.substring(0, 7),
    versionSortBy: x.version_latest ? semverExpand(x.version_latest) : 'Z',
    versionDisplay: x.version_latest ? `v${x.version_latest}` : '-',
    activeAtDisplay: x.active_at_latest
      ? `${formatDistanceToNowStrict(new Date(x.active_at_latest))} ago`
      : '-',
  }));

  return (
    <ClientSearcheableTableWithQueryParamSupport
      data={tableData}
      columns={[
        {
          key: 'bot_name',
          headerText: 'Bot',
          isSortable: true,
        },
        {
          key: 'instance_id',
          isNonRender: true,
        },
        {
          key: 'shortInstanceId',
          headerText: 'ID',
          isSortable: false,
          render: ({ instance_id, shortInstanceId }) => (
            <Cell>
              <Flex inline alignItems={'center'} gap={1} mr={0}>
                <MonoText>{shortInstanceId}</MonoText>
                <CopyButton name={instance_id} />
              </Flex>
            </Cell>
          ),
        },
        {
          key: 'join_method_latest',
          headerText: 'Method',
          isSortable: true,
          render: ({ join_method_latest }) =>
            join_method_latest ? (
              <LabelCell data={[join_method_latest]} />
            ) : (
              <Cell>{'-'}</Cell>
            ),
        },
        {
          key: 'hostnameDisplay',
          headerText: 'Hostname',
          isSortable: true,
        },
        {
          key: 'versionDisplay',
          headerText: 'Version (tbot)',
          isSortable: true,
          altSortKey: 'versionSortBy',
        },
        {
          key: 'activeAtDisplay',
          headerText: 'Last active',
          isSortable: true,
          altSortKey: 'active_at_latest',
        },
      ]}
      emptyText="No active instances found"
      emptyButton={
        <Info alignItems="flex-start" mt={5}>
          Bot instances are ephemeral, and disappear once all issued credentials
          have expired.
        </Info>
      }
      pagination={{ pageSize }}
    />
  );
}

export function semverExpand(version: string) {
  const [major, minor, patchAndRelease] = version.split('.');
  if (!major) {
    return '000000.000000.000000-Z+Z';
  }
  if (!minor) {
    return `${major.padStart(6, '0')}.000000.000000-Z+Z`;
  }
  if (!patchAndRelease) {
    return `${major.padStart(6, '0')}.${minor.padStart(6, '0')}.000000-Z+Z`;
  }
  const [patch, releaseAndBuild = ''] = patchAndRelease.split('-');
  if (!patch) {
    return `major.padStart(6, '0').${minor.padStart(6, '0')}.000000-Z+Z`;
  }
  const [release, build] = releaseAndBuild.split('+');
  return `${major.padStart(6, '0')}.${minor.padStart(6, '0')}.${patch.padStart(6, '0')}-${release || 'Z'}+${build || 'Z'}`;
}
