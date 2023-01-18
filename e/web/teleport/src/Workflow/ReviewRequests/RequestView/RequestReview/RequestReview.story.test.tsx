import React from 'react';

import { render } from 'design/utils/testing';

import { Loaded, Failed } from './RequestReview.story';

test('loaded state', () => {
  const { container } = render(<Loaded />);
  expect(container).toMatchSnapshot();
});

test('failed state', () => {
  const { container } = render(<Failed />);
  expect(container).toMatchSnapshot();
});
