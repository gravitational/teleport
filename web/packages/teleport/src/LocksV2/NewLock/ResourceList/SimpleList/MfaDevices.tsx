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
import { Text } from 'design';
import Table, { Cell } from 'design/DataTable';
import { dateMatcher } from 'design/utils/match';
import { displayDate } from 'shared/services/loc';

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
