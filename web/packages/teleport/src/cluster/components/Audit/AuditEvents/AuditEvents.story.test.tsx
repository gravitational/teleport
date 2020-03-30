import React from 'react';
import { Events } from './AuditEvents.story';
import { render } from 'design/utils/testing';

test('rendering of Events', () => {
  const { container } = render(<Events />);
  expect(container).toMatchSnapshot();
});
