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

import { Box } from 'design';
import Table, { LabelCell } from 'design/DataTable';
import { Indicator } from 'design/Indicator';
import { useInfiniteScroll } from 'shared/hooks';

import { BotOptionsCell } from 'teleport/Bots/List/ActionCell';
import { BotListProps } from 'teleport/Bots/types';

import { DeleteDialog } from '../Delete/DeleteDialog';
import { EditDialog } from '../Edit/EditDialog';
import { ViewBot } from '../ViewBot';

enum Interaction {
  GITHUB_EXAMPLE,
  EDIT,
  DELETE,
  NONE,
}

export function BotList({
  bots,
  disabledEdit,
  disabledDelete,
  onClose,
  onDelete,
  onEdit,
  onSelect,
  selectedBot,
  setSelectedBot,
  hasNextPage,
  isFetchingNextPage,
  onLoadNextPage,
}: BotListProps) {
  const [interaction, setInteraction] = useState<Interaction>(Interaction.NONE);

  const { setTrigger } = useInfiniteScroll({
    fetch: async () => {
      if (hasNextPage && !isFetchingNextPage) {
        onLoadNextPage();
      }
    },
  });

  return (
    <>
      <Table
        data={bots}
        columns={[
          {
            key: 'name',
            headerText: 'Bot Name',
          },
          {
            key: 'roles',
            headerText: 'Roles',
            render: ({ roles }) => <LabelCell data={roles} />,
          },
          {
            altKey: 'options-btn',
            render: bot => (
              <BotOptionsCell
                bot={bot}
                onClickView={() => {
                  setSelectedBot(bot);
                  setInteraction(Interaction.GITHUB_EXAMPLE);
                }}
                disabledEdit={disabledEdit}
                disabledDelete={disabledDelete}
                onClickEdit={() => {
                  if (!disabledEdit) {
                    setSelectedBot(bot);
                    setInteraction(Interaction.EDIT);
                  }
                }}
                onClickDelete={() => {
                  if (!disabledDelete) {
                    setSelectedBot(bot);
                    setInteraction(Interaction.DELETE);
                  }
                }}
              />
            ),
          },
        ]}
        emptyText="No Bots Found"
        row={{
          onClick: onSelect,
          getStyle: () => ({ cursor: 'pointer' }),
        }}
      />
      <Box ref={setTrigger} height="40px" />
      {isFetchingNextPage && (
        <Box textAlign="center">
          <Indicator size={32} delay="none" />
        </Box>
      )}
      {selectedBot && interaction === Interaction.DELETE && (
        <DeleteDialog
          botName={selectedBot.name}
          onCancel={onClose}
          onComplete={onDelete}
          showLockAlternative={false}
        />
      )}
      {selectedBot && interaction === Interaction.EDIT && (
        <EditDialog
          botName={selectedBot.name}
          onCancel={onClose}
          onSuccess={onEdit}
        />
      )}
      {selectedBot && interaction === Interaction.GITHUB_EXAMPLE && (
        <ViewBot onClose={onClose} bot={selectedBot} />
      )}
    </>
  );
}
