import React from 'react';
import { render } from 'design/utils/testing';

import {
  Loaded,
  LoadedFirstTime,
  LoadedNoEmailAddress,
} from './RecoveryNew.story';

test('render screen for recovery tab in account settings', () => {
  const { container } = render(<Loaded />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render screen for first timer recovery tab in account settings', () => {
  const { container } = render(<LoadedFirstTime />);

  expect(container.firstChild).toMatchSnapshot();
});

test('render screen for recovery tab for users without a valid email as username', () => {
  const { container } = render(<LoadedNoEmailAddress />);

  expect(container.firstChild).toMatchSnapshot();
});
