import React from 'react';

import { render, screen } from 'design/utils/testing';

import * as story from './UserTokenLink.story';

jest
  .spyOn(Date, 'now')
  .mockImplementation(() => Date.parse('2021-04-08T07:00:00Z'));

test('reset link dialog as invite', () => {
  render(<story.Invite />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});

test('reset link dialog', () => {
  render(<story.Reset />);
  expect(screen.getByTestId('Modal')).toMatchSnapshot();
});
