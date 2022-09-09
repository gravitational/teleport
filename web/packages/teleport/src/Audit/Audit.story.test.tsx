import React from 'react';
import { render } from 'design/utils/testing';

import { LoadedSample, AllPossibleEvents } from './Audit.story';

test('loaded audit log screen', async () => {
  const { container, findByText } = render(<LoadedSample />);
  await findByText(/Audit Log/);
  expect(container.firstChild).toMatchSnapshot();
});

test('list of all events', async () => {
  const { container } = render(<AllPossibleEvents />);
  expect(container).toMatchSnapshot();
});
