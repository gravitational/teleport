import React from 'react';
import { Loaded, Failed } from './RequestReview.story';
import { render } from 'design/utils/testing';

test('loaded state', () => {
  const { container } = render(<Loaded />);
  expect(container).toMatchSnapshot();
});

test('failed state', () => {
  const { container } = render(<Failed />);
  expect(container).toMatchSnapshot();
});
