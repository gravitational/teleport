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

import { renderHook, waitFor } from '@testing-library/react';

import cfg from 'teleport/config';
// Imports to be mocked
import { fetchClusterAlerts } from 'teleport/services/alerts'; // eslint-disable-line
import useStickyClusterId from 'teleport/useStickyClusterId'; // eslint-disable-line

import { addHours, useAlerts } from './useAlerts';

const ALERTS = [
  {
    kind: 'cluster_alert',
    version: 'v1',
    metadata: {
      name: 'upgrade-suggestion',
      labels: {
        'teleport.internal/alert-on-login': 'yes',
        'teleport.internal/alert-permit-all': 'yes',
      },
      expires: '2022-08-31T17:26:05.728149Z',
    },
    spec: {
      severity: 5,
      message:
        'A new major version of Teleport is available. Please consider upgrading your cluster to v10.',
      created: '2022-08-30T17:26:05.728149Z',
    },
  },
  {
    kind: 'cluster_alert',
    version: 'v1',
    metadata: {
      name: 'license-expired',
      labels: {
        'teleport.internal/alert-on-login': 'yes',
        'teleport.internal/alert-permit-all': 'yes',
        'teleport.internal/link': 'some-URL',
      },
      expires: '2022-08-31T17:26:05.728149Z',
    },
    spec: {
      severity: 5,
      message: 'your license has expired',
      created: '2022-08-30T17:26:05.728149Z',
    },
  },
];

jest.mock('teleport/services/alerts', () => ({
  fetchClusterAlerts: () => Promise.resolve(ALERTS),
}));

jest.mock('teleport/useStickyClusterId', () => () => ({ clusterId: 42 }));

afterEach(() => {
  cfg.isDashboard = false;
});

describe('components/BannerList/useAlerts', () => {
  it('fetches cluster alerts on load', async () => {
    const { result } = renderHook(() => useAlerts());
    await waitFor(() => {
      expect(result.current.alerts).toEqual(ALERTS);
    });
  });

  it('will not return upgrade suggestions on dashboards', async () => {
    cfg.isDashboard = true;
    const { result } = renderHook(() => useAlerts());
    await waitFor(() => {
      const alerts = result.current.alerts;
      alerts.forEach(alert => {
        expect(alert.metadata).not.toBe('upgrade-suggestion');
        expect(alert.metadata).not.toBe('security-patch-available');
      });
    });
  });

  it('provides a method that dismisses alerts for 24h', async () => {
    const { result } = renderHook(() => useAlerts());
    await waitFor(() => {
      expect(result.current.alerts).toEqual(ALERTS);
    });
    result.current.dismissAlert('upgrade-suggestion');

    expect(
      JSON.parse(localStorage.getItem('disabledAlerts'))['upgrade-suggestion']
    ).toBeDefined();
    localStorage.clear();
  });

  it('only returns alerts that are not dismissed', async () => {
    const expireTime = addHours(new Date().getTime(), 24);
    const dismissed = JSON.stringify({
      'upgrade-suggestion': expireTime,
    });
    localStorage.setItem('disabledAlerts', dismissed);

    const { result } = renderHook(() => useAlerts());
    await waitFor(() => {
      expect(result.current.alerts).toEqual(ALERTS.slice(-1));
    });
    localStorage.clear();
  });
});
