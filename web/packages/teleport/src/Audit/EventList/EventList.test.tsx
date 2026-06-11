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

import { screen } from '@testing-library/react';

import { render, userEvent } from 'design/utils/testing';

import { makeEvent } from 'teleport/services/audit';

import EventList from './EventList';

describe('EventList', () => {
  it('renders events using the server-side sort props', () => {
    const event = makeEvent({
      codeDesc: 'Local Login',
      message: 'Local user [root] successfully logged in',
      id: 'user.login:2021-05-25T14:37:27.848Z',
      code: 'T1000I',
      user: 'root',
      time: new Date('2021-05-25T14:37:27.848Z'),
      raw: {
        cluster_name: 'im-a-cluster-name',
        code: 'T1000I',
        ei: 0,
        event: 'user.login',
        method: 'local',
        success: true,
        time: '2021-05-25T14:37:27.848Z',
        user: 'root',
      },
    });

    render(
      <EventList
        events={[event]}
        search=""
        setSearch={() => null}
        sort={{ fieldName: 'time', dir: 'DESC' }}
        setSort={() => null}
      />
    );

    expect(screen.getByText('Local Login')).toBeInTheDocument();
    expect(
      screen.getByText('Local user [root] successfully logged in')
    ).toBeInTheDocument();
  });

  it('delegates sorting to the provided server-side sort handler', async () => {
    const user = userEvent.setup();
    const setSort = jest.fn();

    render(
      <EventList
        events={[]}
        search=""
        setSearch={() => null}
        sort={{ fieldName: 'time', dir: 'DESC' }}
        setSort={setSort}
      />
    );

    await user.click(screen.getByText(/Created \(UTC\)/i));

    expect(setSort).toHaveBeenCalledWith({
      fieldName: 'time',
      dir: 'ASC',
    });
  });
});
