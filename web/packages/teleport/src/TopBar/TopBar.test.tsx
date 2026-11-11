/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import cfg from 'teleport/config';
import { NavigationCategory } from 'teleport/Navigation/categories';
import { NavTitle, type TeleportFeature } from 'teleport/types';

import { defaultLogoHome, getLogoHome } from './TopBar';

const beamsListFeature: TeleportFeature = {
  category: NavigationCategory.Beams,
  hasAccess: () => true,
  route: {
    title: NavTitle.BeamsList,
    path: '/web/beams/list',
    exact: true,
    component: () => null,
  },
};

afterEach(() => {
  jest.restoreAllMocks();
});

describe('getLogoHome', () => {
  test('links to the Beams list when beamsUi is enabled', () => {
    jest.spyOn(cfg, 'getBeamsUi').mockReturnValue(true);

    expect(getLogoHome([beamsListFeature])).toEqual({
      to: '/web/beams/list',
      tooltip: 'Beams',
    });
  });

  test('falls back to root when beamsUi is enabled but Beams list is unavailable', () => {
    jest.spyOn(cfg, 'getBeamsUi').mockReturnValue(true);

    expect(getLogoHome([])).toEqual(defaultLogoHome);
  });

  test('links to root when beamsUi is disabled', () => {
    jest.spyOn(cfg, 'getBeamsUi').mockReturnValue(false);

    expect(getLogoHome([beamsListFeature])).toEqual(defaultLogoHome);
  });
});
