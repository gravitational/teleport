import React from 'react';
import { render } from 'design/utils/testing';

import {
  Processing,
  NoPluginsEnrolled,
  SlackAlreadyEnrolled,
  Failed,
} from './PluginEnroll.story';

test('render Processing', async () => {
  const { container } = render(<Processing />);
  expect(container).toMatchSnapshot();
});

test('render NoPluginsEnrolled', async () => {
  const { container } = render(<NoPluginsEnrolled />);
  expect(container).toMatchSnapshot();
});

test('render SlackAlreadyEnrolled', async () => {
  const { container } = render(<SlackAlreadyEnrolled />);
  expect(container).toMatchSnapshot();
});

test('render Failed', async () => {
  const { container } = render(<Failed />);
  expect(container).toMatchSnapshot();
});
