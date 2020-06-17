import React from 'react';
import { Loaded, Failed } from './Nodes.story';
import { render } from 'design/utils/testing';

test('loaded', () => {
  const { container } = render(<Loaded />);
  expect(container.firstChild).toMatchSnapshot();
});

test('failed', () => {
  const { container } = render(<Failed />);
  expect(container.firstChild).toMatchSnapshot();
});
