import React from 'react';
import {
  LoadedReasonUnrequired,
  Failed,
  Processing,
} from './RequestCreate.story';
import { render } from 'design/utils/testing';

test('loaded state', () => {
  const { container } = render(<LoadedReasonUnrequired />);
  expect(container).toMatchSnapshot();
});

test('failed state', () => {
  const { container } = render(<Failed />);
  expect(container).toMatchSnapshot();
});

test('processing state', () => {
  const { container } = render(<Processing />);
  expect(container).toMatchSnapshot();
});
