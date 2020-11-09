import React from 'react';
import { Loaded, Empty } from './Apps.story';
import { render } from 'design/utils/testing';

test('loaded state', async () => {
  const { container, findAllByText } = render(<Loaded />);
  await findAllByText(/Applications/i);
  expect(container).toMatchSnapshot();
});

test('empty state', async () => {
  const { container, findByText } = render(<Empty />);
  await findByText(/secure your first application/i);

  expect(container).toMatchSnapshot();
});
