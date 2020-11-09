import React from 'react';
import * as story from './UserTokenLink.story';
import { render } from 'design/utils/testing';

test('reset link dialog as invite', () => {
  const { getByTestId } = render(<story.Invite />);
  expect(getByTestId('Modal')).toMatchSnapshot();
});

test('reset link dialog', () => {
  const { getByTestId } = render(<story.Reset />);
  expect(getByTestId('Modal')).toMatchSnapshot();
});
