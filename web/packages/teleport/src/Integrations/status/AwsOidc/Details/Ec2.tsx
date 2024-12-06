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

export function Ec2() {
  return (
    <Table
      data={[]}
      columns={[
        {
          key: 'region',
          headerText: 'Region',
          isSortable: true,
        },
        {
          key: 'labels',
          headerText: 'Labels',
          isSortable: true,
          onSort: (a, b) => {
            const aStr = a.labels.toString();
            const bStr = b.labels.toString();

            if (aStr < bStr) {
              return -1;
            }
            if (aStr > bStr) {
              return 1;
            }

            return 0;
          },
          render: ({ labels }) => <LabelCell data={labels} />,
        },
      ]}
      emptyText="EC2 details coming soon"
      isSearchable
    />
  );
}
