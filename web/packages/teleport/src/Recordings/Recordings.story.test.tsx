import React from 'react';
import { Loaded } from './Recordings.story';
import { render, waitFor } from 'design/utils/testing';

test('rendering of Session Recordings', async () => {
  const { container } = render(<Loaded />);

  await waitFor(() => document.querySelector('table'));
  expect(container).toMatchSnapshot();
});
