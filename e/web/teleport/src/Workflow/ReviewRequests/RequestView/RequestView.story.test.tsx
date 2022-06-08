import React from 'react';
import {
  LoadedSearchPending,
  LoadedRolePending,
  LoadedRoleApproved,
  LoadedRoleDenied,
} from './RequestView.story';
import { render } from 'design/utils/testing';

test('loaded pending role based request state', () => {
  const { container } = render(<LoadedRolePending />);
  expect(container).toMatchSnapshot();
});

test('loaded pending search based request state', () => {
  const { container } = render(<LoadedSearchPending />);
  expect(container).toMatchSnapshot();
});

test('loaded approved role based request state', () => {
  const { container } = render(<LoadedRoleApproved />);
  expect(container).toMatchSnapshot();
});

test('loaded denied role based request state', () => {
  const { container } = render(<LoadedRoleDenied />);
  expect(container).toMatchSnapshot();
});
