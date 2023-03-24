/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';

import { render, screen, fireEvent } from 'design/utils/testing';

import { Tabs } from './Tabs';

test('init tab highlight and content', async () => {
  const { container } = render(<TestTabs />);
  expect(container).toMatchSnapshot();
});

test('clicking on other tabs renders correct content and style', async () => {
  const { container } = render(<TestTabs />);
  fireEvent.click(screen.getByText(/tab two/i));
  expect(container).toMatchSnapshot();
});

const TestTabs = () => (
  <Tabs
    tabs={[
      {
        title: `tab one`,
        content: <div>content 1</div>,
      },
      {
        title: `tab two`,
        content: <div>content 2</div>,
      },
    ]}
  />
);
