/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import styled from 'styled-components';

import { render, screen, userEvent } from 'design/utils/testing';

import { BadgeTitle, ToolTipNoPermBadge } from './ToolTipNoPermBadge';

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

test('badgeTitle prop shows different text', async () => {
  // test default to be BadgeTitle.LackingPermissions
  const { rerender } = render(
    <SomeBox>
      <ToolTipNoPermBadge children="test message" />
    </SomeBox>
  );

  let badge = screen.getByTestId('tooltip');
  expect(badge).toHaveTextContent(BadgeTitle.LackingPermissions);

  // test BadgeTitle.LackingEnterpriseLicense
  rerender(
    <SomeBox>
      <ToolTipNoPermBadge
        children="test message"
        badgeTitle={BadgeTitle.LackingEnterpriseLicense}
      />
    </SomeBox>
  );

  expect(badge).toHaveTextContent(BadgeTitle.LackingEnterpriseLicense);

  // test BadgeTitle.LackingPermissions
  rerender(
    <SomeBox>
      <ToolTipNoPermBadge
        children="test message"
        badgeTitle={BadgeTitle.LackingPermissions}
      />
    </SomeBox>
  );

  expect(badge).toHaveTextContent(BadgeTitle.LackingPermissions);
});

const SomeBox = styled.div`
  width: 240px;
  padding: 16px;
`;
