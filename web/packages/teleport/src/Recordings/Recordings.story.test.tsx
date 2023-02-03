import React from 'react';

import { render, waitFor } from 'design/utils/testing';

import { Loaded } from './Recordings.story';

test('rendering of Session Recordings', async () => {
  const { container } = render(<Loaded />);

  await waitFor(() => document.querySelector('table'));
  expect(container).toMatchSnapshot();
});
