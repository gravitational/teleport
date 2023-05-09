import React from 'react';
import { render } from 'design/utils/testing';

import { Loaded, Failed, Empty, EmptyWithCTA } from './DeviceTrust.story';

test('loaded', () => {
  const { container } = render(<Loaded />);
  expect(container.firstChild).toMatchSnapshot();
});

test('failed', () => {
  const { container } = render(<Failed />);
  expect(container.firstChild).toMatchSnapshot();
});

test('empty state', () => {
  const { container } = render(<Empty />);
  expect(container).toMatchSnapshot();
});

test('empty with CTA', () => {
  const { container } = render(<EmptyWithCTA />);
  expect(container.firstChild).toMatchSnapshot();
});
