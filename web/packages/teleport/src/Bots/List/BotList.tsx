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

import { useState } from 'react';

import Table, { LabelCell } from 'design/DataTable';

import { DeleteBot } from 'teleport/Bots/DeleteBot';
import { EditBot } from 'teleport/Bots/EditBot';
import { BotOptionsCell } from 'teleport/Bots/List/ActionCell';
import { BotListProps } from 'teleport/Bots/types';

import { ViewBot } from '../ViewBot';

enum Interaction {
  VIEW,
  EDIT,
  DELETE,
  NONE,
}

export function BotList({
  attempt,
  bots,
  disabledEdit,
  disabledDelete,
  fetchRoles,
  onClose,
  onDelete,
  onEdit,
  selectedBot,
  setSelectedBot,
  selectedRoles,
  setSelectedRoles,
}: BotListProps) {
  const [interaction, setInteraction] = useState<Interaction>(Interaction.NONE);

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
            onSort: (a, b) =>
              a.roles.toString().localeCompare(b.roles.toString()),
            render: ({ roles }) => <LabelCell data={roles} />,
          },
          {
            altKey: 'options-btn',
            render: bot => (
              <BotOptionsCell
                bot={bot}
                onClickView={() => {
                  setSelectedBot(bot);
                  setInteraction(Interaction.VIEW);
                }}
                disabledEdit={disabledEdit}
                disabledDelete={disabledDelete}
                onClickEdit={() => {
                  setSelectedBot(bot);
                  setSelectedRoles(bot.roles);
                  setInteraction(Interaction.EDIT);
                }}
                onClickDelete={() => {
                  setSelectedBot(bot);
                  setInteraction(Interaction.DELETE);
                }}
              />
            ),
          },
        ]}
        emptyText="No Bots Found"
        isSearchable
        pagination={{ pageSize: 20 }}
      />
      {selectedBot && interaction === Interaction.DELETE && (
        <DeleteBot
          attempt={attempt}
          name={selectedBot.name}
          onClose={onClose}
          onDelete={onDelete}
        />
      )}
      {selectedBot && interaction === Interaction.EDIT && (
        <EditBot
          fetchRoles={fetchRoles}
          attempt={attempt}
          name={selectedBot.name}
          onClose={onClose}
          onEdit={onEdit}
          selectedRoles={selectedRoles}
          setSelectedRoles={setSelectedRoles}
        />
      )}
      {selectedBot && interaction === Interaction.VIEW && (
        <ViewBot onClose={onClose} bot={selectedBot} />
      )}
    </>
  );
}
