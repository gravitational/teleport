import React from 'react';
import { Loaded } from './Recordings.story';
import { render, waitForElement } from 'design/utils/testing';

test('rendering of Session Recordings', async () => {
  const { container } = render(<Loaded />);

  await waitForElement(() => document.querySelector('table'));
  expect(container).toMatchSnapshot();
});
