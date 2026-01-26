/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { act, renderHook } from '@testing-library/react';
import { useNavigate } from 'react-router';

import { NavigationSection, NavigationSubsection } from './Navigation';
import { useDefaultNavigation } from './useDefaultNavigation';

jest.mock('react-router', () => ({
  useNavigate: jest.fn(),
}));

const navigateMock = jest.fn();

describe('useDefaultNavigation', () => {
  beforeEach(() => {
    (useNavigate as jest.Mock).mockReturnValue(navigateMock);
  });

  it('returns an onClick function that calls the first section onclick and navigates to its route', () => {
    const sectionOnclickMock = jest.fn();
    const testRoute = '/test';

    const section = {
      subsections: [
        {
          title: 'test',
          exact: true,
          icon: () => null,
          route: testRoute,
          onClick: sectionOnclickMock,
        },
        {
          title: 'test2',
          exact: true,
          icon: () => null,
          route: testRoute + '2',
        },
      ] as NavigationSubsection[],
    } as NavigationSection;

    const { result } = renderHook(() => useDefaultNavigation(section));

    act(() => {
      result.current.onClick?.();
    });

    expect(sectionOnclickMock).toHaveBeenCalled();
    expect(navigateMock).toHaveBeenCalledWith(testRoute);
  });
});
