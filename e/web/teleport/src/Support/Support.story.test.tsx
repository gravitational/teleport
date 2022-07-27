import React from 'react';
import { render } from 'design/utils/testing';
import { SupportCloud } from 'teleport/Support/Support.story';

test('render support cloud section', async () => {
  const { container } = render(<SupportCloud />);
  expect(container.firstChild).toMatchSnapshot();
});
