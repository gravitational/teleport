import React from 'react';
import { render } from 'design/utils/testing';

import { Loaded, Failed, Empty, EmptyReadOnly } from './Nodes.story';

test('loaded', () => {
  const { container } = render(<Loaded />);
  expect(container.firstChild).toMatchSnapshot();
});

test('failed', () => {
  const { container } = render(<Failed />);
  expect(container.firstChild).toMatchSnapshot();
});

test('empty state', () => {
  const { container } = render(<Empty />);
  expect(container).toMatchSnapshot();
});

test('readonly empty state', () => {
  const { container } = render(<EmptyReadOnly />);
  expect(container).toMatchSnapshot();
});
