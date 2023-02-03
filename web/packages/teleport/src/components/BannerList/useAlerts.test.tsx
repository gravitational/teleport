import { renderHook } from '@testing-library/react-hooks';

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

describe('components/BannerList/useAlerts', () => {
  it('fetches cluster alerts on load', async () => {
    const { result, waitFor } = renderHook(() => useAlerts());
    await waitFor(() => {
      expect(result.current.alerts).toEqual(ALERTS);
    });
  });

  it('provides a method that dismisses alerts for 24h', async () => {
    const { result, waitFor } = renderHook(() => useAlerts());
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

    const { result, waitFor } = renderHook(() => useAlerts());
    await waitFor(() => {
      expect(result.current.alerts).toEqual(ALERTS.slice(-1));
    });
    localStorage.clear();
  });
});
