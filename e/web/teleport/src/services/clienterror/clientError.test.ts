import cfg from 'e-teleport/config';
import api from 'teleport/services/api';

import {
  reportClientError,
  REPORT_WINDOW_MS,
  type ClientErrorComponent,
} from './clientError';

// Each test starts on a fresh, widely-spaced system time so the module-level
// throttle/dedup state from a previous test has always aged out of its
// window (60s) by the time the next test runs.
let currentTime = 0;

beforeEach(() => {
  jest.useFakeTimers();
  currentTime += 10 * REPORT_WINDOW_MS;
  jest.setSystemTime(currentTime);
  jest.spyOn(api, 'fetch').mockResolvedValue({} as Response);
  cfg.oss.isCloud = true;
  cfg.oss.isUsageBasedBilling = false;
});

afterEach(() => {
  jest.useRealTimers();
  jest.clearAllMocks();
  cfg.oss.isCloud = false;
  cfg.oss.isUsageBasedBilling = false;
});

test('sends a report with only component and error_source', () => {
  reportClientError('cloud-panel', 'network', 'failed to load');

  expect(api.fetch).toHaveBeenCalledTimes(1);
  expect(api.fetch).toHaveBeenCalledWith(cfg.api.logClientErrorPath, {
    method: 'POST',
    body: JSON.stringify({
      client_component: 'cloud-panel',
      error_source: 'network',
    }),
  });
});

test('throttles after 5 calls within the window', () => {
  for (let i = 0; i < 5; i++) {
    reportClientError('cloud-panel', 'network', `error-${i}`);
  }
  expect(api.fetch).toHaveBeenCalledTimes(5);

  reportClientError('cloud-panel', 'network', 'error-6th');
  expect(api.fetch).toHaveBeenCalledTimes(5);
});

test('allows calls again once the throttle window passes', () => {
  for (let i = 0; i < 5; i++) {
    reportClientError('cloud-panel', 'network', `error-${i}`);
  }
  expect(api.fetch).toHaveBeenCalledTimes(5);

  jest.setSystemTime(currentTime + REPORT_WINDOW_MS + 1000);
  reportClientError('cloud-panel', 'network', 'error-after-window');

  expect(api.fetch).toHaveBeenCalledTimes(6);
});

test('deduplicates an identical fingerprint within the window', () => {
  reportClientError('cloud-panel', 'network', 'same error');
  reportClientError('cloud-panel', 'network', 'same error');

  expect(api.fetch).toHaveBeenCalledTimes(1);
});

test('does not deduplicate the same error signature across different components', () => {
  const otherComponent = 'other-component' as ClientErrorComponent;

  reportClientError('cloud-panel', 'network', 'same error');
  reportClientError(otherComponent, 'network', 'same error');

  expect(api.fetch).toHaveBeenCalledTimes(2);
});

test('does not deduplicate the same error signature across different sources', () => {
  reportClientError('cloud-panel', 'network', 'same error');
  reportClientError('cloud-panel', 'render', 'same error');

  expect(api.fetch).toHaveBeenCalledTimes(2);
});

test('resends an identical fingerprint once the dedup window passes', () => {
  reportClientError('cloud-panel', 'network', 'same error');
  jest.setSystemTime(currentTime + REPORT_WINDOW_MS + 1000);
  reportClientError('cloud-panel', 'network', 'same error');

  expect(api.fetch).toHaveBeenCalledTimes(2);
});

test('does not report when neither cloud nor usage-based billing is enabled', () => {
  cfg.oss.isCloud = false;
  cfg.oss.isUsageBasedBilling = false;

  reportClientError('cloud-panel', 'network', 'ineligible tenant');

  expect(api.fetch).not.toHaveBeenCalled();
});

test('reports when usage-based billing is enabled even if isCloud is false', () => {
  cfg.oss.isCloud = false;
  cfg.oss.isUsageBasedBilling = true;

  reportClientError('cloud-panel', 'network', 'dashboard tenant');

  expect(api.fetch).toHaveBeenCalledTimes(1);
});
