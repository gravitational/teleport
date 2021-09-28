import React from 'react';
import { render } from 'design/utils/testing';
import * as WithMulti from './Multi.story';
import * as WithPassword from './Password.story';
import * as WithTotp from './Totp.story';
import * as WithU2f from './U2f.story';

test('render form for authenticating with password to recover mfa device', () => {
  const { container } = render(<WithPassword.Loaded />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render failed state form for authenticating with password', () => {
  const { container } = render(<WithPassword.Failed />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render form for authenticating with totp', () => {
  const { container } = render(<WithTotp.Loaded />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render failed state form for authenticating with totp', () => {
  const { container } = render(<WithTotp.Failed />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render form for authenticating with u2f', () => {
  const { container } = render(<WithU2f.Loaded />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render failed state form for authenticating with u2f', () => {
  const { container } = render(<WithU2f.Failed />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render form for authenticating with either totp or u2f', () => {
  const { container } = render(<WithMulti.Loaded />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render failed state form for authenticating with either totp or u2f', () => {
  const { container } = render(<WithMulti.Failed />);

  expect(container.firstChild).toMatchSnapshot();
});
