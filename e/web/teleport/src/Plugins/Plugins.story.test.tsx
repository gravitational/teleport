import React from 'react';
import { render } from 'design/utils/testing';

import { Processing, Loaded, Empty, Failed } from './Plugins.story';

test('render Processing', async () => {
  const { container } = render(<Processing />);
  expect(container).toMatchSnapshot();
});

test('render Loaded', async () => {
  const { container } = render(<Loaded />);
  expect(container).toMatchSnapshot();
});

test('render Empty', async () => {
  const { container } = render(<Empty />);
  expect(container).toMatchSnapshot();
});

test('render Failed', async () => {
  const { container } = render(<Failed />);
  expect(container).toMatchSnapshot();
});
