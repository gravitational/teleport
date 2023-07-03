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
import Table from 'design/DataTable';

import { Resource, KindRole } from 'teleport/services/resources';

import { renderActionCell, SimpleListProps } from '../common';

export function Roles(
  props: SimpleListProps & { roles: Resource<KindRole>[] }
) {
  const {
    roles = [],
    selectedResources,
    toggleSelectResource,
    fetchStatus,
  } = props;

  return (
    <Table
      data={roles}
      columns={[
        {
          key: 'name',
          headerText: 'Name',
        },
        {
          altKey: 'action-btn',
          render: ({ name }) =>
            renderActionCell(Boolean(selectedResources.role[name]), () =>
              toggleSelectResource({ kind: 'role', targetValue: name })
            ),
        },
      ]}
      emptyText="No Roles Found"
      pagination={{ pageSize: props.pageSize }}
      isSearchable
      fetching={{
        fetchStatus,
      }}
    />
  );
}
