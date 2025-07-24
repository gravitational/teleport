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

import { renderHook } from '@testing-library/react';
import { PropsWithChildren } from 'react';

import { TeleportProviderBasic } from './mocks/providers';
import { useClusterVersion } from './useClusterVersion';

describe('useClusterVersion', () => {
  it('should return the cluster version (auth)', () => {
    const { result } = renderHook(() => useClusterVersion(), {
      wrapper: Wrapper,
    });
    expect(result.current.clusterVersion?.version).toBe('4.4.0-dev');
  });

  it.each`
    version      | diff
    ${'4.4.0'}   | ${'n'}
    ${'4.4.1'}   | ${'n'}
    ${'4.3.999'} | ${'n*'}
    ${'4.3.0'}   | ${'n*'}
    ${'5.0.0'}   | ${'n+1'}
    ${'6.0.0'}   | ${'n+'}
    ${'3.0.0'}   | ${'n-1'}
    ${'2.0.0'}   | ${'n-'}
  `('diff("$version") should be "$diff"', ({ version, diff }) => {
    const { result } = renderHook(() => useClusterVersion(), {
      wrapper: Wrapper,
    });
    expect(result.current.diff(version)).toBe(diff);
  });
});

function Wrapper(props: PropsWithChildren) {
  return (
    <TeleportProviderBasic>
      <div>{props.children}</div>
    </TeleportProviderBasic>
  );
}
