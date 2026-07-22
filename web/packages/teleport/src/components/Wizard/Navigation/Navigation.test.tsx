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

import { render, screen } from 'design/utils/testing';

import { Navigation } from './Navigation';

const steps = [{ title: 'first' }, { title: 'second' }, { title: 'third' }];

test('step 1/3', async () => {
  render(<Navigation views={steps} currentStep={0} />);

  const firstBullet = screen.getByTestId('bullet-active');
  expect(firstBullet).toHaveTextContent('');
  expect(firstBullet.parentElement).toHaveTextContent(/first/i);

  const uncheckedBullets = screen.getAllByTestId('bullet-default');
  expect(uncheckedBullets).toHaveLength(2);

  // second bullet
  expect(uncheckedBullets[0]).toHaveTextContent(/2/i);
  expect(uncheckedBullets[0].parentElement).toHaveTextContent(/second/i);

  // last bullet
  expect(uncheckedBullets[1]).toHaveTextContent(/3/i);
  expect(uncheckedBullets[1].parentElement).toHaveTextContent(/third/i);
});

test('step 2/3', async () => {
  render(<Navigation views={steps} currentStep={1} />);

  const firstBullet = screen.getByTestId('bullet-checked');
  expect(firstBullet).toHaveTextContent('');
  expect(firstBullet.parentElement).toHaveTextContent(/first/i);

  const secondBullet = screen.getByTestId('bullet-active');
  expect(secondBullet).toHaveTextContent('');
  expect(secondBullet.parentElement).toHaveTextContent(/second/i);

  const lastBullet = screen.getByTestId('bullet-default');
  expect(lastBullet).toHaveTextContent(/3/i);
  expect(lastBullet.parentElement).toHaveTextContent(/third/i);
});

test('step 3/3', async () => {
  render(<Navigation views={steps} currentStep={2} />);

  const checkedBullets = screen.getAllByTestId('bullet-checked');
  expect(checkedBullets).toHaveLength(2);

  // first bullet
  expect(checkedBullets[0]).toHaveTextContent('');
  expect(checkedBullets[0].parentElement).toHaveTextContent(/first/i);

  // second bullet
  expect(checkedBullets[1]).toHaveTextContent('');
  expect(checkedBullets[1].parentElement).toHaveTextContent(/second/i);

  // last bullet
  const lastBullet = screen.getByTestId('bullet-active');
  expect(lastBullet).toHaveTextContent('');
  expect(lastBullet.parentElement).toHaveTextContent(/third/i);
});
