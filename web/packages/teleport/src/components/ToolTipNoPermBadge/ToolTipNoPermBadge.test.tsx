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

import { ToolTipNoPermBadge, TooltipText } from './ToolTipNoPermBadge';

test('hovering renders tooltip msg and unhovering makes it disappear', async () => {
  render(
    <SomeBox>
      <ToolTipNoPermBadge children="test message" />
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
      <ToolTipNoPermBadge children="test message" sticky={true} />
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

test('tooltipText prop shows different text', async () => {
  // test default to be TooltipText.LackingPermissions
  const { rerender } = render(
    <SomeBox>
      <ToolTipNoPermBadge children="test message" />
    </SomeBox>
  );

  let badge = screen.getByTestId('tooltip');
  expect(badge).toHaveTextContent(TooltipText.LackingPermissions);

  // test TooltipText.LackingEnterpriseLicense
  rerender(
    <SomeBox>
      <ToolTipNoPermBadge
        children="test message"
        tooltipText={TooltipText.LackingEnterpriseLicense}
      />
    </SomeBox>
  );

  expect(badge).toHaveTextContent(TooltipText.LackingEnterpriseLicense);

  // test TooltipText.LackingPermissions
  rerender(
    <SomeBox>
      <ToolTipNoPermBadge
        children="test message"
        tooltipText={TooltipText.LackingPermissions}
      />
    </SomeBox>
  );

  expect(badge).toHaveTextContent(TooltipText.LackingPermissions);
});

const SomeBox = styled.div`
  width: 240px;
  padding: 16px;
`;
