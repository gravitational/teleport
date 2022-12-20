import React from 'react';
import { render, screen } from 'design/utils/testing';

import { Loaded, Failed, Empty, EmptyReadOnly } from './Apps.story';

jest.mock('teleport/useStickyClusterId', () =>
  jest.fn(() => ({ clusterId: 'im-a-cluster', isLeafCluster: false }))
);

test('loaded state', async () => {
  const { container } = render(<Loaded />);
  await screen.findAllByText(/Applications/i);

  expect(container).toMatchSnapshot();
});

test('failed state', async () => {
  const { container } = render(<Failed />);
  await screen.findAllByText(/some error message/i);

  expect(container).toMatchSnapshot();
});

test('empty state for enterprise, can create', () => {
  const { container } = render(<Empty />);
  expect(container).toMatchSnapshot();
});

test('readonly empty state', () => {
  const { container } = render(<EmptyReadOnly />);
  expect(container).toMatchSnapshot();
});
