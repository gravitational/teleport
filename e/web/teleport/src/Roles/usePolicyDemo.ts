import { useCallback, useEffect } from 'react';

import { Attempt, useAsync } from 'shared/hooks/useAsync';

import { accessGraphService } from 'e-teleport/services/accessgraph';
import { AccessGraphSettings } from 'e-teleport/services/accessgraph/accessgraph';
import { Cfg } from 'teleport/config';
import { RoleDiffState } from 'teleport/Roles/Roles';
import { ApiError } from 'teleport/services/api/parseError';
import { storageService } from 'teleport/services/storageService';

export const WAIT_FOR_SYNC_MAX_TRIES = 10;
export const WAIT_FOR_SYNC_TIMEOUT = 2000;

type UsePolicyDemoState = {
  roleTesterEnabled: boolean;
  isCloud: boolean;
  state: RoleDiffState;
  enableDemoMode: () => void;
  errorMessage: string;
};

function getDemoState({
  accessGraphSettingsAttempt,
  enableDemoModeAttempt,
  waitingForSyncAttempt,
  roleTesterEnabled,
  isCloud,
}: {
  isCloud: boolean;
  roleTesterEnabled: boolean;
  accessGraphSettingsAttempt: Attempt<AccessGraphSettings>;
  enableDemoModeAttempt: Attempt<boolean>;
  waitingForSyncAttempt: Attempt<boolean>;
}) {
  if (roleTesterEnabled) {
    return RoleDiffState.PolicyEnabled;
  }
  // the data returned from this attempt is just a boolean. There is a possibility that it returns false,
  // so we cannot just check for the data to exist or for the attempt to succeed.
  if (enableDemoModeAttempt.data) {
    return RoleDiffState.DemoReady;
  }
  if (
    accessGraphSettingsAttempt.data?.enable_demo_mode &&
    accessGraphSettingsAttempt.data?.status?.initial_sync_complete &&
    accessGraphSettingsAttempt.data?.status?.http_ready
  ) {
    return RoleDiffState.DemoReady;
  }
  if (!roleTesterEnabled && !isCloud) {
    return RoleDiffState.Disabled;
  }
  if (
    accessGraphSettingsAttempt.status === 'error' ||
    waitingForSyncAttempt.status === 'error'
  ) {
    return RoleDiffState.Error;
  }
  if (accessGraphSettingsAttempt.status === 'processing') {
    return RoleDiffState.LoadingSettings;
  }
  // these states are the same, waitingForSyncAttempt is only used when the page loads after demo mode
  // has been enabled but we are still waiting for sync
  if (
    waitingForSyncAttempt.status === 'processing' ||
    enableDemoModeAttempt.status === 'processing'
  ) {
    return RoleDiffState.WaitingForSync;
  }
  return RoleDiffState.Disabled;
}

export function usePolicyDemo(cfg: Cfg): UsePolicyDemoState {
  const roleTesterEnabled =
    cfg.isPolicyEnabled &&
    cfg.isPolicyRoleVisualizerEnabled &&
    storageService.getAccessGraphRoleTesterEnabled();

  const isCloud = cfg.isCloud;

  const [accessGraphSettingsAttempt, getAccessGraphSettings] = useAsync(
    useCallback(async () => {
      return await accessGraphService.getAccessGraphSettings();
    }, [])
  );

  const waitForInitialSyncComplete = useCallback(async (tries: number) => {
    if (tries !== 0) {
      await new Promise(resolve => setTimeout(resolve, WAIT_FOR_SYNC_TIMEOUT)); // wait for 2 seconds after initial attempt
    }
    if (tries > WAIT_FOR_SYNC_MAX_TRIES) {
      throw new Error(
        'Initial resource sync is taking longer than expected. Please try again in a few minutes.'
      );
    }
    try {
      const settings = await accessGraphService.getAccessGraphSettings();
      if (
        settings.status?.initial_sync_complete &&
        settings.status?.http_ready
      ) {
        return true; // demo mode is ready
      }
      return await waitForInitialSyncComplete(tries + 1);
    } catch (err) {
      if (err instanceof ApiError && err.response.status === 404) {
        // if they hit a different version proxy in the load balancer, lets just try again and hopefully get lucky
        return await waitForInitialSyncComplete(tries + 1);
      }
      throw new Error(err);
    }
  }, []);

  const [waitingForSyncAttempt, waitForSync] = useAsync(
    waitForInitialSyncComplete
  );

  const [enableDemoModeAttempt, enableDemoMode] = useAsync(
    useCallback(async () => {
      // use one abortSignal for the entire retry chain
      try {
        await accessGraphService.enableDemoMode();
      } catch (err) {
        throw new Error('Failed to enable demo mode', { cause: err });
      }

      return await waitForInitialSyncComplete(0);
    }, [waitForInitialSyncComplete])
  );

  useEffect(() => {
    // if the roleTester is enabled, that means they have full access and we dont need to check settings.
    if (!isCloud || roleTesterEnabled) {
      return;
    }
    // If roleTester isnt enabled but its cloud, we need to check settings to see if demo mode is active.
    // on load, we want to grab the settings. There is a chance that they enabled demo mode
    // and something went wrong with the sync (or it took too long) and they are refreshing the page.
    async function getInitialSyncIfDemoModeEnabled() {
      const [settings, error] = await getAccessGraphSettings();
      if (
        !error &&
        settings.enable_demo_mode &&
        !settings.status?.initial_sync_complete
      ) {
        waitForSync(0);
      }
    }
    getInitialSyncIfDemoModeEnabled();
  }, [getAccessGraphSettings, waitForSync, isCloud, roleTesterEnabled]);

  return {
    roleTesterEnabled,
    isCloud,
    enableDemoMode,
    errorMessage:
      accessGraphSettingsAttempt.statusText ||
      enableDemoModeAttempt.statusText ||
      waitingForSyncAttempt.statusText,
    state: getDemoState({
      waitingForSyncAttempt,
      enableDemoModeAttempt,
      accessGraphSettingsAttempt,
      isCloud,
      roleTesterEnabled,
    }),
  };
}
