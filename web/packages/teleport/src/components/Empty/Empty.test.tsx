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

import { render, screen } from 'design/utils/testing';

import Empty, { Props } from './Empty';

test('empty state for enterprise or oss, with create perms', async () => {
  render(
    <MemoryRouter>
      <Empty {...props} />
    </MemoryRouter>
  );

  await expect(
    screen.findByText(/Add your first Linux server to Teleport/i)
  ).resolves.toBeVisible();
});

test('empty state for cant create or leaf cluster', async () => {
  render(
    <MemoryRouter>
      <Empty {...props} canCreate={false} />
    </MemoryRouter>
  );

  await expect(
    screen.findByText(/Either there are no servers in the/i)
  ).resolves.toBeVisible();
});

const props: Props = {
  clusterId: 'im-a-cluster',
  canCreate: true,
  emptyStateInfo: {
    title: 'Add your first Linux server to Teleport',
    byline:
      'Teleport Server Access consolidates SSH access across all environments.',
    docsURL:
      'https://goteleport.com/docs/enroll-resources/server-access/getting-started/',
    readOnly: {
      title: 'No Servers Found',
      resource: 'servers',
    },
  },
};
