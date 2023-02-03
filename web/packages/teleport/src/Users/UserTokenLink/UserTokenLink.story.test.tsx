import React from 'react';

import { render } from 'design/utils/testing';

import * as story from './UserTokenLink.story';

jest
  .spyOn(Date, 'now')
  .mockImplementation(() => Date.parse('2021-04-08T07:00:00Z'));

test('reset link dialog as invite', () => {
  const { getByTestId } = render(<story.Invite />);
  expect(getByTestId('Modal')).toMatchSnapshot();
});

test('reset link dialog', () => {
  const { getByTestId } = render(<story.Reset />);
  expect(getByTestId('Modal')).toMatchSnapshot();
});
