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

import { render, screen } from 'design/utils/testing';

import { BotList } from 'teleport/Bots/List/BotList';
import { BotListProps } from 'teleport/Bots/types';
import { botsFixture } from 'teleport/Bots/fixtures';

const makeProps = (): BotListProps => ({
  attempt: { status: '' },
  bots: botsFixture,
  disabledEdit: false,
  onClose: () => { },
  onDelete: () => { },
  onEdit: () => { },
  roles: [],
  selectedBot: null,
  selectedRoles: [],
  setSelectedBot: () => { },
  onView: () => { },
  setSelectedRoles: () => { },
});

test('renders table with bots', () => {
  const props = makeProps();
  render(<BotList {...props} />);

  const rows = screen.getAllByRole('row');
  expect(rows).toHaveLength(props.bots.length + 1);

  props.bots.forEach(row => {
    expect(screen.getByText(row.name)).toBeInTheDocument();
    row.roles.forEach(role => {
      expect(screen.getByText(role)).toBeInTheDocument();
    });
  });
});
