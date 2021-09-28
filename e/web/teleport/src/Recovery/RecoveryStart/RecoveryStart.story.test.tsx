import React from 'react';
import { render } from 'design/utils/testing';
import { ForgotPassword, ForgotMfa } from './RecoveryStart.story';

test('render correct form for clicking forgot password', () => {
  const { container } = render(<ForgotPassword />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render correct form for clicking forgot mfa device', () => {
  const { container } = render(<ForgotMfa />);

  expect(container.firstChild).toMatchSnapshot();
});
