import React from 'react';
import { Sessions } from './AuditSessions.story';
import { render } from 'design/utils/testing';

test('rendering of Audit Sessions', () => {
  const { container, queryByText } = render(<Sessions />);
  expect(queryByText('no-display')).toBeNull();
  expect(container).toMatchSnapshot();
});
