/**
 * Copyright 2023 Gravitational, Inc.
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
import { ClickableLabelCell } from 'design/DataTable';

import { Desktop } from 'teleport/services/desktops';

import { ServerSideListProps, StyledTable, renderActionCell } from '../common';

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
