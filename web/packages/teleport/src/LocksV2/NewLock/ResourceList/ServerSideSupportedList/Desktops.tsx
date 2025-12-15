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

import { ClickableLabelCell } from 'design/DataTable';

import { Desktop } from 'teleport/services/desktops';

import { renderActionCell, ServerSideListProps, StyledTable } from '../common';

export function Desktops(props: ServerSideListProps & { desktops: Desktop[] }) {
  const {
    desktops = [],
    selectedResources,
    customSort,
    onLabelClick,
    toggleSelectResource,
  } = props;

  return (
    <StyledTable
      data={desktops}
      columns={[
        {
          key: 'addr',
          headerText: 'Address',
        },
        {
          key: 'name',
          headerText: 'Name',
          isSortable: true,
        },
        {
          key: 'labels',
          headerText: 'Labels',
          render: ({ labels }) => (
            <ClickableLabelCell labels={labels} onClick={onLabelClick} />
          ),
        },
        {
          altKey: 'action-btn',
          render: agent =>
            renderActionCell(
              Boolean(selectedResources.windows_desktop[agent.name]),
              () =>
                toggleSelectResource({
                  kind: 'windows_desktop',
                  targetValue: agent.name,
                })
            ),
        },
      ]}
      emptyText="No Desktops Found"
      customSort={customSort}
      disableFilter
      fetching={{
        fetchStatus: props.fetchStatus,
      }}
    />
  );
}
