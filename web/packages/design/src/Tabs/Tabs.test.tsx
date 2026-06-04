/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { render, screen, userEvent } from 'design/utils/testing';

import { TabBorder, TabContainer, TabsContainer } from './Tabs';

test('enabled tab calls onClick when clicked', async () => {
  const user = userEvent.setup();
  const onClick = jest.fn();
  render(
    <TabsContainer>
      <TabContainer onClick={onClick}>Enabled Tab</TabContainer>
      <TabBorder />
    </TabsContainer>
  );

  const tab = screen.getByText('Enabled Tab');
  expect(tab).not.toHaveAttribute('aria-disabled');
  await user.click(tab);
  expect(onClick).toHaveBeenCalledTimes(1);
});

test('disabled tab does not call onClick when clicked', async () => {
  const user = userEvent.setup();
  const onClick = jest.fn();
  render(
    <TabsContainer>
      <TabContainer disabled onClick={onClick}>
        Disabled Tab
      </TabContainer>
      <TabBorder />
    </TabsContainer>
  );

  const tab = screen.getByText('Disabled Tab');
  expect(tab).toHaveAttribute('aria-disabled', 'true');
  await user.click(tab);
  expect(onClick).not.toHaveBeenCalled();
});

// Disabled tabs keep pointer events enabled so hover-based
// behavior like hover tooltips still works.
test('disabled tab keeps pointer events', () => {
  render(
    <TabsContainer>
      <TabContainer disabled>Disabled Tab</TabContainer>
      <TabBorder />
    </TabsContainer>
  );

  expect(screen.getByText('Disabled Tab')).not.toHaveStyle({
    'pointer-events': 'none',
  });
});
