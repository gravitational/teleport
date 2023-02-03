import React from 'react';
import { render } from 'design/utils/testing';
import { MemoryRouter } from 'react-router';

import Empty, { Props } from './Empty';

test('empty state for enterprise or oss, with create perms', async () => {
  const { findByText } = render(
    <MemoryRouter>
      <Empty {...props} />
    </MemoryRouter>
  );

  await expect(
    findByText(/Add your first Linux server to Teleport/i)
  ).resolves.toBeVisible();
});

test('empty state for cant create or leaf cluster', async () => {
  const { findByText } = render(
    <MemoryRouter>
      <Empty {...props} canCreate={false} />
    </MemoryRouter>
  );

  await expect(
    findByText(/Either there are no servers in the/i)
  ).resolves.toBeVisible();
});

const props: Props = {
  clusterId: 'im-a-cluster',
  canCreate: true,
  emptyStateInfo: {
    title: 'Add your first Linux server to Teleport',
    byline:
      'Teleport Server Access consolidates SSH access across all environments.',
    docsURL: 'https://goteleport.com/docs/server-access/getting-started/',
    resourceType: 'server',
    readOnly: {
      title: 'No Servers Found',
      resource: 'servers',
    },
  },
};
