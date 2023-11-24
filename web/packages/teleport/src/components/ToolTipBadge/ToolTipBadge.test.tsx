/**
 * Copyright 2023 Gravitational, Inc.
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
import styled from 'styled-components';

import { render, screen, userEvent } from 'design/utils/testing';

import { ToolTipBadge } from './ToolTipBadge';

test('hovering renders tooltip msg and unhovering makes it disappear', async () => {
  render(
    <SomeBox>
      <ToolTipBadge
        color="#007D6B"
        badgeTitle="Title"
        children="test message"
      />
    </SomeBox>
  );

  expect(screen.queryByTestId('tooltip-msg')).not.toBeInTheDocument();

  const badge = screen.getByTestId('tooltip');

  await userEvent.hover(badge);
  expect(screen.getByTestId('tooltip-msg')).toBeInTheDocument();

  await userEvent.unhover(badge);
  expect(screen.queryByTestId('tooltip-msg')).not.toBeInTheDocument();
});

test('sticky prop prevents tooltip from disappearing until child element is unhovered', async () => {
  render(
    <SomeBox>
      <ToolTipBadge
        color="#007D6B"
        badgeTitle="Title"
        children="test message"
        sticky={true}
      />
    </SomeBox>
  );

  expect(screen.queryByTestId('tooltip-msg')).not.toBeInTheDocument();

  const badge = screen.getByTestId('tooltip');

  await userEvent.hover(badge);
  expect(screen.getByTestId('tooltip-msg')).toBeInTheDocument();

  const badgeChild = screen.getByTestId('tooltip-msg');

  // tooltip should be open on unhover
  await userEvent.unhover(badge);
  expect(screen.getByTestId('tooltip-msg')).toBeInTheDocument();

  // tooltip dissapears on child unhover
  await userEvent.unhover(badgeChild);
  expect(screen.queryByTestId('tooltip-msg')).not.toBeInTheDocument();
});

const SomeBox = styled.div`
  width: 240px;
  padding: 16px;
`;
