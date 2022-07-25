import React from 'react';

import { render } from 'design/utils/testing';

import { Loaded } from './RequestList.story';

test('loaded state', () => {
  const { container } = render(<Loaded />);
  expect(container).toMatchSnapshot();
});
