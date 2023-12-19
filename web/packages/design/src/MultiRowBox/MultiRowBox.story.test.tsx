import React from 'react';

import { render } from 'design/utils/testing';

import { WithMultipleRows, WithSingleRow } from './MultiRowBox.story';

test('renders single row', () => {
  const { container } = render(<WithSingleRow />);
  expect(container.firstChild).toMatchSnapshot();
});

test('renders multiple rows', () => {
  const { container } = render(<WithMultipleRows />);
  expect(container.firstChild).toMatchSnapshot();
});
