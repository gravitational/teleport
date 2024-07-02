import React from 'react';

import { render } from 'design/utils/testing';

import { Buttons } from './buttons.story';

test('buttons', () => {
  const { container } = render(<Buttons />);
  expect(container).toMatchSnapshot();
});
