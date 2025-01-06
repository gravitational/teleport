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

import { addIndexToViews, computeViewChildrenSize } from './flow';
import { Navigation } from './Navigation';

test('computeViewChildrenSize', async () => {
  const nestedViews = [
    {
      title: 'Ridiculous',
      views: [
        {
          title: 'Nesting',
          views: [
            {
              title: 'Here',
            },
            {
              title: 'Again',
            },
          ],
        },
      ],
    },
    {
      title: 'Banana',
      hide: true,
    },
  ];
  expect(computeViewChildrenSize({ views: nestedViews })).toBe(3);
  expect(
    computeViewChildrenSize({ views: nestedViews, constrainToVisible: true })
  ).toBe(2);

  const notNestedViews = [
    {
      title: 'Apple',
    },
    {
      title: 'Banana',
    },
  ];
  expect(computeViewChildrenSize({ views: notNestedViews })).toBe(2);
});

test('addIndexToViews and rendering correct steps', async () => {
  const nestedViews = [
    {
      title: 'First Step',
    },
    {
      title: 'Nesting',
      views: [
        {
          title: 'Nesting Again',
          views: [
            {
              title: 'Second Step',
            },
            {
              title: 'Third Step',
            },
          ],
        },
        {
          title: 'Fourth Step',
        },
      ],
    },
    {
      title: 'Fifth Step',
    },
  ];

  const indexedViews = addIndexToViews(nestedViews);

  // Should render 5 bullets.
  render(<Navigation views={indexedViews} currentStep={0} />);

  // First bullets always active.
  const firstBullet = screen.getByTestId('bullet-active');
  expect(firstBullet).toHaveTextContent('');
  expect(firstBullet.parentElement).toHaveTextContent(/first step/i);

  // Rest should be not active.
  const uncheckedBullets = screen.getAllByTestId('bullet-default');
  expect(uncheckedBullets).toHaveLength(4);

  expect(uncheckedBullets[0]).toHaveTextContent(/2/i);
  expect(uncheckedBullets[0].parentElement).toHaveTextContent(/second/i);

  expect(uncheckedBullets[1]).toHaveTextContent(/3/i);
  expect(uncheckedBullets[1].parentElement).toHaveTextContent(/third/i);

  expect(uncheckedBullets[2]).toHaveTextContent(/4/i);
  expect(uncheckedBullets[2].parentElement).toHaveTextContent(/fourth/i);

  expect(uncheckedBullets[3]).toHaveTextContent(/5/i);
  expect(uncheckedBullets[3].parentElement).toHaveTextContent(/fifth/i);
});
