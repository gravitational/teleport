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

import { ButtonBorder, ButtonPrimary } from 'design/Button';
import Table, { Cell } from 'design/DataTable';

export function Roles(props: {
  requestable: string[];
  requested: Set<string>;
  onToggleRole(role: string): void;
  /** Disables buttons.*/
  disabled?: boolean;
}) {
  const addToRequestText = props.requested.size
    ? '+ Add to Request'
    : '+ Request Access';

  return (
    <Table
      data={props.requestable.map(role => ({ role }))}
      pagination={{ pagerPosition: 'top', pageSize: 10 }}
      isSearchable={true}
      columns={[
        {
          key: 'role',
          headerText: 'Role Name',
          isSortable: true,
        },
        {
          altKey: 'action',
          render: ({ role }) => {
            const isAdded = props.requested.has(role);
            const commonProps = {
              disabled: props.disabled,
              width: '137px',
              size: 'small' as const,
              onClick: () => props.onToggleRole(role),
            };
            return (
              <Cell align="right">
                {isAdded ? (
                  <ButtonPrimary {...commonProps}>Remove</ButtonPrimary>
                ) : (
                  <ButtonBorder {...commonProps}>
                    {addToRequestText}
                  </ButtonBorder>
                )}
              </Cell>
            );
          },
        },
      ]}
      emptyText="No Requestable Roles Found"
    />
  );
}
