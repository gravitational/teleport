import React from 'react';
import { render } from 'design/utils/testing';
import Empty, { Props } from './Empty';

test('empty state for enterprise or oss, with create perms', async () => {
  const { findByText } = render(<Empty {...props} />);

  await expect(
    findByText(/Add your first Linux server to Teleport/i)
  ).resolves.toBeVisible();
});

test('empty state for cant create or leaf cluster', async () => {
  const { findByText } = render(<Empty {...props} canCreate={false} />);

  await expect(
    findByText(/Either there are no servers in the/i)
  ).resolves.toBeVisible();
});

const props: Props = {
  clusterId: 'im-a-cluster',
  canCreate: true,
  onClick: () => null,
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
