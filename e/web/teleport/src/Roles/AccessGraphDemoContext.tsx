import {
  createContext,
  FC,
  PropsWithChildren,
  useCallback,
  useContext,
  useEffect,
  useMemo,
} from 'react';

import { Attempt, useAsync } from 'shared/hooks/useAsync';

import { accessGraphService } from 'e-teleport/services/accessgraph';
import { AccessGraphSettings } from 'e-teleport/services/accessgraph/accessgraph';
import osCfg from 'teleport/config';
import { RoleDiffState } from 'teleport/Roles/Roles';
import { ApiError } from 'teleport/services/api/parseError';
import { storageService } from 'teleport/services/storageService';

export const WAIT_FOR_SYNC_MAX_TRIES = 10;
export const WAIT_FOR_SYNC_TIMEOUT = 2000;

type AccessGraphDemoContextState = {
  roleTesterEnabled: boolean;
  isCloud: boolean;
  /**
   * State of the access graph, largely if its ready for use
   * or disabled.
   */
  state: RoleDiffState;
  /**
   * Only use if "roleTesterEnabled" is not enabled, but
   * user wants to try out the demo version of access graph.
   */
  enableDemoMode: () => void;
  errorMessage: string;
};

const AccessGraphDemoContext = createContext<AccessGraphDemoContextState>(null);

/**
 * Provider for determining access graph setting and its state.
 *
 * If the cluster is not cloud or policy with access graph is enabled, this
 * provider doesn't do much other than set the access graph "state" to
 * "Disabled" or "PolicyEnabled".
 *
 * If the cluster is cloud and policy is NOT enabled*, then this provider will
 * attempt to determine if demo access graph is `enabled`. If enabled this
 * provider will sync with the backend to see if the demo access graph is
 * ready to be used (set state to "DemoReady"). It will retry until it's
 * ready or has reached maximum retry.
 *
 * *To be considered "enabled", the demo access graph has to be enabled
 * directly from cluster user eg: user clicks on "Enable demo mode" btn in
 * a role visualizer.
 */
export const AccessGraphDemoProvider: FC<PropsWithChildren> = ({
  children,
}) => {
  /**
   * There are two similar `access graph demo enabled` flags:
   *   - One flag is the "feature" flag (you have to enable it in salescenter
   *     feature checkbox), and you get this flag from entitlements (this flag)
   *   - Second flag is "user" enabled access graph setting "enable_demo_mode".
   *     This is when user clicks on "Preview Identity" which sets this flag
   *     to true.
   * The feature has to be enabled if the user wants to preview access graph
   * regardless if user enabled it. A user could've enabled it if they had
   * this feature before, then lost it. Turning off the feature does not
   * turn off the user setting.
   */
  const featureAccessGraphDemoEnabled =
    osCfg.entitlements.AccessGraphDemoMode.enabled;

  const roleTesterEnabled =
    osCfg.entitlements.AccessGraph.enabled &&
    osCfg.isPolicyRoleVisualizerEnabled &&
    storageService.getAccessGraphRoleTesterEnabled();

  const isCloud = osCfg.isCloud;

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
        'Failed previewing access graph: Initial resource sync is taking longer than expected. Please try again in a few minutes.'
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
      throw err;
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
    if (!isCloud || roleTesterEnabled || !featureAccessGraphDemoEnabled) {
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

  const demoState = useMemo(
    () =>
      getDemoState({
        waitingForSyncAttempt,
        enableDemoModeAttempt,
        accessGraphSettingsAttempt,
        isCloud,
        roleTesterEnabled,
      }),
    [
      waitingForSyncAttempt,
      enableDemoModeAttempt,
      accessGraphSettingsAttempt,
      isCloud,
      roleTesterEnabled,
    ]
  );

  const errorMessage =
    (accessGraphSettingsAttempt.status === 'error'
      ? accessGraphSettingsAttempt.statusText
      : '') ||
    (enableDemoModeAttempt.status === 'error'
      ? enableDemoModeAttempt.statusText
      : '') ||
    (waitingForSyncAttempt.status === 'error'
      ? waitingForSyncAttempt.statusText
      : '');

  const values = {
    roleTesterEnabled,
    isCloud,
    enableDemoMode,
    errorMessage,
    state: demoState,
  };

  return (
    <AccessGraphDemoContext.Provider value={values}>
      {children}
    </AccessGraphDemoContext.Provider>
  );
};

/**
 * useAccessGraphDemo allows enabling demo mode and see the state
 * of the access graph.
 */
export function useAccessGraphDemo() {
  const context = useContext(AccessGraphDemoContext);

  if (!context) {
    throw new Error(
      'useAccessGraphDemo must be used within a AccessGraphDemoProvider'
    );
  }

  return context;
}

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

  // Prioritize processing states. The error could persist.
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

  if (
    accessGraphSettingsAttempt.status === 'error' ||
    waitingForSyncAttempt.status === 'error'
  ) {
    return RoleDiffState.Error;
  }
  return RoleDiffState.Disabled;
}
