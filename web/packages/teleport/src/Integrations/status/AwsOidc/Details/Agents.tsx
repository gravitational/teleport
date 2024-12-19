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

import React from 'react';

import Table, { LabelCell } from 'design/DataTable';

export function Agents() {
  return (
    <Table
      data={[]}
      columns={[
        {
          key: 'name',
          headerText: 'Service Name',
          isSortable: true,
        },
        {
          key: 'region',
          headerText: 'Region',
          isSortable: true,
        },
        {
          key: 'tags',
          headerText: 'Tags',
          isSortable: true,
          onSort: (a, b) => {
            const aStr = a.tags.toString();
            const bStr = b.tags.toString();

            if (aStr < bStr) {
              return -1;
            }
            if (aStr > bStr) {
              return 1;
            }

            return 0;
          },
          render: ({ tags }) => <LabelCell data={tags} />,
        },
      ]}
      emptyText="Agents details coming soon"
      isSearchable
    />
  );
}
