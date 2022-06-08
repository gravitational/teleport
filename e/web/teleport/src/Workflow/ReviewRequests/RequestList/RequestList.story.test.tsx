import React from 'react';
import { Loaded } from './RequestList.story';
import { render } from 'design/utils/testing';

test('loaded state', () => {
  const { container } = render(<Loaded />);
  expect(container).toMatchSnapshot();
});
