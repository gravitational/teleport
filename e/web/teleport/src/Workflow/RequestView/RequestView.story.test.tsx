import React from 'react';
import {
  LoadedPending,
  LoadedApproved,
  LoadedDenied,
  Failed,
  Processing,
} from './RequestView.story';
import { render } from 'design/utils/testing';

test('loaded pending request state', () => {
  const { container } = render(<LoadedPending />);
  expect(container).toMatchSnapshot();
});

test('loaded approved request state', () => {
  const { container } = render(<LoadedApproved />);
  expect(container).toMatchSnapshot();
});

test('loaded denied request state', () => {
  const { container } = render(<LoadedDenied />);
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
