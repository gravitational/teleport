import React from 'react';
import { Overflow } from './Audit.story';
import { render } from 'design/utils/testing';

test('overflow', async () => {
  const { container, findByText } = render(<Overflow />);
  await findByText(/exceeded the maximum limit of 9999 events/);

  expect(container.firstChild).toMatchSnapshot();
});
