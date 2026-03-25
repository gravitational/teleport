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
    expect(result.current.clusterVersion).toBe('4.4.0-dev');
  });

  it.each`
    clientVersion  | compatibility
    ${'4.4.0-dev'} | ${{ isCompatible: true, reason: 'match' }}
    ${'4.4.0'}     | ${{ isCompatible: false, reason: 'too-new' }}
    ${'4.4.1'}     | ${{ isCompatible: false, reason: 'too-new' }}
    ${'4.3.999'}   | ${{ isCompatible: true, reason: 'upgrade-minor' }}
    ${'4.3.0'}     | ${{ isCompatible: true, reason: 'upgrade-minor' }}
    ${'5.0.0'}     | ${{ isCompatible: false, reason: 'too-new' }}
    ${'3.0.0'}     | ${{ isCompatible: true, reason: 'upgrade-major' }}
    ${'2.0.0'}     | ${{ isCompatible: false, reason: 'too-old' }}
  `(
    'diff("$clientVersion") should be "$compatibility"',
    ({ clientVersion, compatibility }) => {
      const { result } = renderHook(() => useClusterVersion(), {
        wrapper: Wrapper,
      });
      expect(result.current.checkCompatibility(clientVersion)).toStrictEqual(
        compatibility
      );
    }
  );
});

function Wrapper(props: PropsWithChildren) {
  return (
    <TeleportProviderBasic>
      <div>{props.children}</div>
    </TeleportProviderBasic>
  );
}
