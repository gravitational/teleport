import React from 'react';
import { render } from 'design/utils/testing';

import {
  Apps,
  Databases,
  Desktops,
  Kubes,
  Nodes,
  Roles,
  UserGroups,
} from './ResourceList.story';

test('render Apps', async () => {
  const { container } = render(<Apps />);
  expect(container).toMatchSnapshot();
});

test('render Databases', async () => {
  const { container } = render(<Databases />);
  expect(container).toMatchSnapshot();
});

test('render Desktops', async () => {
  const { container } = render(<Desktops />);
  expect(container).toMatchSnapshot();
});

test('render Kubes', async () => {
  const { container } = render(<Kubes />);
  expect(container).toMatchSnapshot();
});

test('render Nodes', async () => {
  const { container } = render(<Nodes />);
  expect(container).toMatchSnapshot();
});

test('render Roles', async () => {
  const { container } = render(<Roles />);
  expect(container).toMatchSnapshot();
});

test('render UserGroups', async () => {
  const { container } = render(<UserGroups />);
  expect(container).toMatchSnapshot();
});
