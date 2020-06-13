import React from 'react';
import { Loaded, Failed } from './Nodes.story';
import { render, Router } from 'design/utils/testing';

test('loaded', () => {
  const { container } = render(
    <Router>
      <Loaded />
    </Router>
  );
  expect(container.firstChild).toMatchSnapshot();
});

test('failed', () => {
  const { container } = render(
    <Router>
      <Failed />
    </Router>
  );
  expect(container.firstChild).toMatchSnapshot();
});
