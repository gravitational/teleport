import { act, renderHook, waitFor } from '@testing-library/react';

import { accessGraphService } from 'e-teleport/services/accessgraph';
import cfg from 'teleport/config';
import { RoleDiffState } from 'teleport/Roles/Roles';
import { ApiError } from 'teleport/services/api/parseError';
import { storageService } from 'teleport/services/storageService';

import {
  AccessGraphDemoProvider,
  useAccessGraphDemo,
  WAIT_FOR_SYNC_MAX_TRIES,
  WAIT_FOR_SYNC_TIMEOUT,
} from './AccessGraphDemoContext';

jest.mock('teleport/services/storageService', () => ({
  storageService: {
    getAccessGraphRoleTesterEnabled: jest.fn(),
    getUseNewRoleEditor: jest.fn(),
  },
}));

beforeEach(() => {
  jest.resetAllMocks();
  cfg.isPolicyEnabled = false;
  cfg.isPolicyRoleVisualizerEnabled = true;
  cfg.isCloud = true;
});

const wrapper = ({ children }) => (
  <AccessGraphDemoProvider>{children}</AccessGraphDemoProvider>
);

test('should return DISABLED state when not in cloud', () => {
  cfg.isCloud = false;
  const { result } = renderHook(() => useAccessGraphDemo(), {
    wrapper,
  });

  expect(result.current.state).toBe(RoleDiffState.Disabled);
  expect(result.current.isCloud).toBe(false);
});

test('should return POLICY_ENABLED state when role tester is enabled', async () => {
  cfg.isPolicyEnabled = true;
  (storageService.getAccessGraphRoleTesterEnabled as jest.Mock).mockReturnValue(
    true
  );

  const { result } = renderHook(() => useAccessGraphDemo(), {
    wrapper,
  });
  await waitFor(() => {
    expect(result.current.state).toBe(RoleDiffState.PolicyEnabled);
  });
  expect(result.current.roleTesterEnabled).toBe(true);
});

test('should return DEMO_READY state when demo mode is enabled and sync is complete', async () => {
  jest.spyOn(accessGraphService, 'getAccessGraphSettings').mockResolvedValue({
    enable_demo_mode: true,
    status: {
      initial_sync_complete: true,
      http_ready: true,
    },
  });

  const { result } = renderHook(() => useAccessGraphDemo(), {
    wrapper,
  });

  await waitFor(() => {
    expect(result.current.state).toBe(RoleDiffState.DemoReady);
  });
});

test('should return WAITING_FOR_SYNC state when enabling demo mode', async () => {
  jest.spyOn(accessGraphService, 'getAccessGraphSettings').mockResolvedValue({
    enable_demo_mode: true,
    status: {
      initial_sync_complete: false,
      http_ready: false,
    },
  });

  const { result } = renderHook(() => useAccessGraphDemo(), {
    wrapper,
  });

  await act(async () => {
    result.current.enableDemoMode();
  });

  await waitFor(() => {
    expect(result.current.state).toBe(RoleDiffState.WaitingForSync);
  });
});

test('should handle error states correctly', async () => {
  const error = new ApiError({
    message: 'error here',
    response: { status: 403 } as Response,
  });

  jest
    .spyOn(accessGraphService, 'getAccessGraphSettings')
    .mockRejectedValue(new ApiError(error));

  const { result } = renderHook(() => useAccessGraphDemo(), {
    wrapper,
  });

  await waitFor(() => {
    expect(result.current.state).toBe(RoleDiffState.Error);
  });
  expect(result.current.errorMessage).toBe('error here');
});

describe('waitForInitialSync', () => {
  test('should succeed after receiving a successful initial_sync_complete', async () => {
    jest.useFakeTimers();
    jest
      .spyOn(accessGraphService, 'getAccessGraphSettings')
      .mockResolvedValue({
        enable_demo_mode: true,
        status: {
          initial_sync_complete: false,
          http_ready: false,
        },
      })
      .mockResolvedValue({
        enable_demo_mode: true,
        status: {
          initial_sync_complete: false,
          http_ready: false,
        },
      })
      .mockResolvedValue({
        enable_demo_mode: true,
        status: {
          initial_sync_complete: true,
          http_ready: true,
        },
      });

    const { result } = renderHook(() => useAccessGraphDemo(), {
      wrapper,
    });

    await act(async () => {
      result.current.enableDemoMode();
    });

    await waitFor(() => {
      expect(result.current.state).toBe(RoleDiffState.DemoReady);
    });
    jest.useRealTimers();
  });

  test('should fail instead of retry if receiving a non-404 status code', async () => {
    const error = new ApiError({
      message: 'error here',
      response: { status: 403 } as Response,
    });

    jest
      .spyOn(accessGraphService, 'getAccessGraphSettings')
      .mockResolvedValue({
        enable_demo_mode: true,
        status: {
          initial_sync_complete: false,
          http_ready: false,
        },
      })
      .mockRejectedValue(error);

    const { result } = renderHook(() => useAccessGraphDemo(), {
      wrapper,
    });

    await act(async () => {
      result.current.enableDemoMode();
    });
    await waitFor(() => {
      expect(result.current.state).toBe(RoleDiffState.Error);
    });
    expect(result.current.errorMessage).toBe('error here');
  });

  test('should fail after max retries have been hit', async () => {
    jest.useFakeTimers();
    let spyOn = jest.spyOn(accessGraphService, 'getAccessGraphSettings');
    for (let i = 0; i < WAIT_FOR_SYNC_MAX_TRIES; i++) {
      spyOn = spyOn.mockResolvedValue({
        enable_demo_mode: true,
        status: {
          initial_sync_complete: false,
          http_ready: false,
        },
      });
    }

    const { result } = renderHook(() => useAccessGraphDemo(), {
      wrapper,
    });

    await act(async () => {
      result.current.enableDemoMode();
    });

    for (let i = 0; i < WAIT_FOR_SYNC_MAX_TRIES + 1; i++) {
      act(() => {
        jest.advanceTimersByTime(WAIT_FOR_SYNC_TIMEOUT);
      });

      // this is to flush any remaining promises. It could be left empty
      // but the linter complains about empty `act` calls, so resolve a promise instead
      await act(async () => {
        return Promise.resolve(true);
      });
    }

    await waitFor(() => {
      expect(result.current.state).toBe(RoleDiffState.Error);
    });
    expect(result.current.errorMessage).toBe('Failed to enable demo mode');
    jest.useRealTimers();
  });
});
