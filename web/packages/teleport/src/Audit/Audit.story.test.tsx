import React from 'react';
import * as Stories from './Audit.story';
import { render } from 'design/utils/testing';

test('overflow', async () => {
  const { container, findByText } = render(<Stories.Overflow />);
  await findByText(/exceeded the maximum limit of 9999 events/);

  expect(container.firstChild).toMatchSnapshot();
});

test('loaded', async () => {
  const { container, findByText } = render(<Stories.Loaded />);
  await findByText(/SHOWING/);
  expect(container.firstChild).toMatchSnapshot();
});
