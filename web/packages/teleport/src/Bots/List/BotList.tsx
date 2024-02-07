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

import Table, { LabelCell } from 'design/DataTable';

import React from 'react';

import { BotOptionsCell } from 'teleport/Bots/List/ActionCell';

import { BotListProps } from 'teleport/Bots/types';
import { DeleteBot } from 'teleport/Bots/DeleteBot';

export function BotList({
  attempt,
  bots,
  onClose,
  onDelete,
  selectedBot,
  setSelectedBot,
  onView,
}: BotListProps) {
  return (
    <>
      <Table
        data={bots}
        columns={[
          {
            key: 'name',
            headerText: 'Bot Name',
            isSortable: true,
          },
          {
            key: 'roles',
            headerText: 'Roles',
            isSortable: true,
            onSort: (a: string[], b: string[]) =>
              a.toString().localeCompare(b.toString()),
            render: ({ roles }) => <LabelCell data={roles} />,
          },
          {
            altKey: 'options-btn',
            render: bot => (
              <BotOptionsCell
                bot={bot}
                onClickDelete={() => setSelectedBot(bot)}
                onClickView={() => onView(bot)}
              />
            ),
          },
        ]}
        emptyText="No Bots Found"
        isSearchable
        pagination={{ pageSize: 20 }}
      />
      {selectedBot && (
        <DeleteBot
          attempt={attempt}
          name={selectedBot.name}
          onClose={onClose}
          onDelete={onDelete}
        />
      )}
    </>
  );
}
