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

import { Text } from 'design';
import Table, { Cell } from 'design/DataTable';
import { displayDate } from 'design/datetime';
import { dateMatcher } from 'design/utils/match';

import { MfaDevice } from 'teleport/services/mfa/types';

import { renderActionCell, SimpleListProps } from '../common';

export function MfaDevices(
  props: SimpleListProps & { mfaDevices: MfaDevice[] }
) {
  const {
    mfaDevices = [],
    selectedResources,
    toggleSelectResource,
    fetchStatus,
  } = props;

  return (
    <Table
      data={mfaDevices}
      columns={[
        {
          key: 'description',
          headerText: 'Type',
        },
        {
          key: 'name',
          headerText: 'Device Name',
          render: renderNameCell,
        },
        {
          key: 'registeredDate',
          headerText: 'Registered',
          isSortable: true,
          render: ({ registeredDate }) => (
            <Cell>{displayDate(registeredDate)}</Cell>
          ),
        },
        {
          key: 'lastUsedDate',
          headerText: 'Last Used',
          isSortable: true,
          render: ({ lastUsedDate }) => (
            <Cell>{displayDate(lastUsedDate)}</Cell>
          ),
        },
        {
          altKey: 'action-btn',
          render: ({ name, id }) =>
            renderActionCell(Boolean(selectedResources.mfa_device[id]), () =>
              toggleSelectResource({
                kind: 'mfa_device',
                targetValue: id,
                friendlyName: name,
              })
            ),
        },
      ]}
      emptyText="No Devices Found"
      isSearchable
      initialSort={{
        key: 'registeredDate',
        dir: 'DESC',
      }}
      customSearchMatchers={[dateMatcher(['registeredDate', 'lastUsedDate'])]}
      pagination={{ pageSize: props.pageSize }}
      fetching={{
        fetchStatus,
      }}
    />
  );
}

const renderNameCell = ({ name }: MfaDevice) => {
  return (
    <Cell title={name}>
      <Text
        style={{
          maxWidth: '96px',
          whiteSpace: 'nowrap',
        }}
      >
        {name}
      </Text>
    </Cell>
  );
};
