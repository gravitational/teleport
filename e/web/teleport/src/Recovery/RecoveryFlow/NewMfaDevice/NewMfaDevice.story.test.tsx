import React from 'react';
import { render } from 'design/utils/testing';

import {
  LoadedMulti,
  LoadedWebauthn,
  LoadedTotp,
  Failed,
} from './NewMfaDevice.story';

test('render correct form to add with multi option', () => {
  const { container } = render(<LoadedMulti />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render correct form to add TOTP device', () => {
  const { container } = render(<LoadedTotp />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render correct form to add Webauthn device', () => {
  const { container } = render(<LoadedWebauthn />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render failed state for form to add with multi option', () => {
  const { container } = render(<Failed />);

  expect(container.firstChild).toMatchSnapshot();
});
