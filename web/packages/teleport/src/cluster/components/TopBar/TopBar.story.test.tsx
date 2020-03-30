import React from 'react';
import { ClusterTopBar } from './TopBar.story';
import { render } from 'design/utils/testing';

test('rendering of ClusterTopBar', () => {
  const { container } = render(<ClusterTopBar />);
  expect(container.firstChild).toMatchSnapshot();
});
