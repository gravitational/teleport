/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { render, screen, within } from 'design/utils/testing';

import { User } from 'teleport/services/user';

import { UserDetails, UserDetailsTitle } from './UserDetails';

jest.mock('teleport/lib/locks/useResourceLock', () => ({
  useResourceLock: () => ({ isLocked: false, isLoading: false }),
}));

describe('UserDetails display names', () => {
  it('renders display values in the title and keeps the username field canonical', () => {
    const user: User = {
      name: 'alice',
      roles: ['access'],
      authType: 'local',
      isLocal: true,
      displayPrimary: 'Alice Jones',
      displaySecondary: 'alice@example.com',
    };

    render(<UserDetailsTitle user={user} />);

    expect(screen.getByText('Alice Jones')).toBeInTheDocument();
    expect(screen.getByText('alice@example.com')).toBeInTheDocument();
    expect(
      screen.getByLabelText('Alice Jones, alice@example.com, username alice')
    ).toBeInTheDocument();
    expect(screen.queryByText('alice')).not.toBeInTheDocument();

    render(<UserDetails user={user} sections={[]} />);

    const usernameField = screen.getByText('Username')
      .parentElement as HTMLElement;
    expect(within(usernameField).getByText('alice')).toBeInTheDocument();
    // Auth Type is in the body, with the resource icon in front.
    const authTypeField = screen.getByText('Auth Type')
      .parentElement as HTMLElement;
    expect(within(authTypeField).getByText('local')).toBeInTheDocument();
    expect(
      within(authTypeField).getByTestId('res-icon-server')
    ).toBeInTheDocument();
    expect(screen.getByText('Status')).toBeInTheDocument();
  });

  it('renders secondary-only users with username as the primary value', () => {
    const user: User = {
      name: 'alice',
      roles: ['access'],
      authType: 'local',
      isLocal: true,
      displaySecondary: 'alice@example.com',
    };

    render(<UserDetailsTitle user={user} />);

    expect(screen.getByText('alice')).toBeInTheDocument();
    expect(screen.getByText('alice@example.com')).toBeInTheDocument();
  });

  it('renders username-only users without duplicating the username', () => {
    const user: User = {
      name: 'bot-user',
      roles: ['bot-user'],
      authType: 'local',
      isLocal: true,
      isBot: true,
    };

    render(<UserDetailsTitle user={user} />);

    expect(screen.queryAllByText('bot-user')).toHaveLength(1);
  });
});
