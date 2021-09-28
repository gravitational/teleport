import React from 'react';
import { render } from 'design/utils/testing';
import { FromInvite, FromReset } from './RecoveryCodes.story';

test('render correct dialog after creating a new account', () => {
  const { container } = render(<FromInvite />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render correct dialog after resetting account', () => {
  const { container } = render(<FromReset />);

  expect(container.firstChild).toMatchSnapshot();
});
