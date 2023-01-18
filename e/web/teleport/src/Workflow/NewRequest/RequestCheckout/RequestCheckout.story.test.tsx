import React from 'react';
import { render } from 'design/utils/testing';

import { Loaded, Failed, Success } from './RequestCheckout.story';

test('loaded state', async () => {
  const { container } = render(<Loaded />);
  expect(container).toMatchSnapshot();
});

test('failed state', async () => {
  const { container } = render(<Failed />);
  expect(container).toMatchSnapshot();
});

test('success state', () => {
  const { container } = render(<Success />);
  expect(container).toMatchSnapshot();
});
