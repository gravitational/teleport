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

import { MemoryRouter } from 'react-router';

import { botsFixture } from 'teleport/Bots/fixtures';
import { BotList } from 'teleport/Bots/List/BotList';

import { EmptyState } from './EmptyState/EmptyState';

export default {
  title: 'Teleport/Bots',
};

export const Empty = () => {
  return (
    <MemoryRouter>
      <EmptyState />
    </MemoryRouter>
  );
};

export const List = () => {
  return (
    <BotList
      attempt={{ status: '' }}
      bots={botsFixture}
      disabledEdit={false}
      disabledDelete={false}
      onClose={() => {}}
      onDelete={() => {}}
      onEdit={() => {}}
      fetchRoles={async () => []}
      selectedBot={null}
      selectedRoles={[]}
      setSelectedBot={() => {}}
      setSelectedRoles={() => {}}
    />
  );
};
