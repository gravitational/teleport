import React from 'react';
import { render, screen } from 'design/utils/testing';

import { LoadedSample, AllPossibleEvents } from './Audit.story';

test('loaded audit log screen', async () => {
  const { container } = render(<LoadedSample />);
  await screen.findByText(/Audit Log/);
  expect(container.firstChild).toMatchSnapshot();
});

test('list of all events', async () => {
  const { container } = render(<AllPossibleEvents />);
  expect(container).toMatchSnapshot();
});
