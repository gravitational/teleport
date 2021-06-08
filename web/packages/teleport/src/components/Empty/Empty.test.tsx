import React from 'react';
import { Text, Link } from 'design';
import { render } from 'design/utils/testing';
import Empty, { Props } from './Empty';

test('empty state for enterprise or oss, with create perms', async () => {
  const { findByText } = render(<Empty {...props} />);

  expect(await findByText(/ADD YOUR FIRST SERVER/i)).toBeVisible();
});

test('empty state for cant create or leaf cluster', async () => {
  const { findByText } = render(<Empty {...props} canCreate={false} />);

  expect(await findByText(/There are no servers for the/i)).toBeVisible();
});

const props: Props = {
  clusterId: 'im-a-cluster',
  canCreate: true,
  onClick: () => null,
  emptyStateInfo: {
    title: 'ADD YOUR FIRST SERVER',
    description: (
      <Text>
        Consolidate access to databases running behind NAT, prevent data
        exfiltration, meet compliance requirements, and have complete visibility
        into access and behavior.{' '}
        <Link
          target="_blank"
          href="https://goteleport.com/docs/database-access/guides/"
        >
          Follow the documentation
        </Link>{' '}
        to get started.
      </Text>
    ),
    buttonText: 'ADD SERVER',
    videoLink: 'https://www.youtube.com/watch?v=tUXYtwP-Kvw',
    readOnly: {
      title: 'No Servers Found',
      message: 'There are no servers for the "',
    },
  },
};
